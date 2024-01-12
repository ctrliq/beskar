// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package gossip

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/netip"
	"os"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/google/uuid"
	"go.ciq.dev/beskar/pkg/mtls"
	"go.ciq.dev/beskar/pkg/netutil"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var runInKubernetes = os.Getenv("KUBERNETES_SERVICE_HOST") != ""

const (
	GossipLabelKey = "go.ciq.dev/beskar-gossip"
	namespaceFile  = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

func Start(gossipConfig Config, meta *BeskarMeta, client kubernetes.Interface, timeout time.Duration) (*Member, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	peers, err := getPeers(gossipConfig, client, timeout)
	if err != nil {
		return nil, err
	}
	key, err := getKey(gossipConfig)
	if err != nil {
		return nil, err
	}
	nodeMeta, err := meta.Encode()
	if err != nil {
		return nil, err
	}
	state, err := getState(len(peers))
	if err != nil {
		return nil, err
	}

	host, port, err := net.SplitHostPort(gossipConfig.Addr)
	if err != nil {
		return nil, err
	} else if host == "" {
		host = "0.0.0.0"
	} else if host == "[::]" || host == "::" {
		ips, err := netutil.LocalOutboundIPs()
		if err != nil {
			return nil, err
		}

		if len(ips) > 0 {
			host = ips[0].String()
		}
	}

	return NewMember(
		id.String(),
		peers,
		WithBindAddress(net.JoinHostPort(host, port)),
		WithSecretKey(key),
		WithNodeMeta(nodeMeta),
		WithLocalState(state),
	)
}

func getKey(gossipConfig Config) ([]byte, error) {
	return base64.StdEncoding.DecodeString(gossipConfig.Key)
}

func getState(numPeers int) ([]byte, error) {
	if numPeers == 0 || !runInKubernetes {
		caCert, caKey, err := mtls.GenerateCA("beskar", time.Now().AddDate(10, 0, 0), mtls.ECDSAKey)
		if err != nil {
			return nil, err
		}
		return mtls.MarshalCAPEM(&mtls.CAPEM{
			Cert: caCert,
			Key:  caKey,
		})
	}
	return nil, nil
}

func getPeers(gossipConfig Config, client kubernetes.Interface, timeout time.Duration) ([]string, error) {
	if !runInKubernetes {
		return gossipConfig.Peers, nil
	}

	data, err := os.ReadFile(namespaceFile)
	if err != nil {
		return nil, err
	}

	namespace := string(bytes.TrimSpace(data))

	if client == nil {
		inCluster, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("while getting k8s cluster configuration: %w", err)
		}
		client, err = kubernetes.NewForConfig(inCluster)
		if err != nil {
			return nil, fmt.Errorf("while instantiating k8s client: %w", err)
		}
	}

	podIP, err := netutil.RouteGetSourceAddress(os.Getenv("KUBERNETES_SERVICE_HOST"))
	if err != nil {
		return nil, err
	}

	var peers []string

	getPeers := func() error {
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
		defer cancel()

		endpointList, err := client.CoreV1().Endpoints(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.Set(map[string]string{
				GossipLabelKey: "true",
			}).String(),
		})
		if err != nil {
			return fmt.Errorf("while listing endpoints: %w", err)
		}

		var subsetIPs []netip.AddrPort
		gossipPort := uint16(0)

		for _, ep := range endpointList.Items {
			for _, subset := range ep.Subsets {
				for _, port := range subset.Ports {
					if port.Protocol != v1.ProtocolTCP {
						continue
					}
					gossipPort = uint16(port.Port)
					break
				}
				for _, address := range subset.Addresses {
					addr, err := netip.ParseAddr(address.IP)
					if err != nil {
						continue
					}
					subsetIPs = append(subsetIPs, netip.AddrPortFrom(addr, gossipPort))
				}
			}
		}

		if gossipPort == 0 {
			return fmt.Errorf("no gossip port found")
		}

		peers = nil

		for _, ip := range subsetIPs {
			if ip.Addr().String() == podIP {
				continue
			}
			peers = append(peers, ip.String())
		}

		if len(subsetIPs) == 0 {
			return fmt.Errorf("no gossip peer found")
		}

		return nil
	}

	eb := backoff.NewExponentialBackOff()
	eb.MaxElapsedTime = timeout

	return peers, backoff.RetryNotify(getPeers, eb, func(err error, backoff time.Duration) {})
}
