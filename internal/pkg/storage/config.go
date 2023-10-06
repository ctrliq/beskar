// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package storage

const (
	FSStorageDriver    = "filesystem"
	S3StorageDriver    = "s3"
	GCSStorageDriver   = "gcs"
	AzureStorageDriver = "azure"
)

type S3StorageConfig struct {
	Endpoint        string `yaml:"endpoint"`
	Bucket          string `yaml:"bucket"`
	AccessKeyID     string `yaml:"access-key-id"`
	SecretAccessKey string `yaml:"secret-access-key"`
	SessionToken    string `yaml:"session-token"`
	Region          string `yaml:"region"`
	DisableSSL      bool   `yaml:"disable-ssl"`
}

type FSStorageConfig struct {
	Directory string `yaml:"directory"`
}

type GCSStorageConfig struct {
	Bucket  string `yaml:"bucket"`
	Keyfile string `yaml:"keyfile"`
}

type AzureStorageConfig struct {
	Container   string `yaml:"container"`
	AccountName string `yaml:"account-name"`
	AccountKey  string `yaml:"account-key"`
}

type Config struct {
	Driver     string             `yaml:"driver"`
	Prefix     string             `yaml:"prefix"`
	S3         S3StorageConfig    `yaml:"s3"`
	Filesystem FSStorageConfig    `yaml:"filesystem"`
	GCS        GCSStorageConfig   `yaml:"gcs"`
	Azure      AzureStorageConfig `yaml:"azure"`
}
