FROM rockylinux:8-minimal as Builder

RUN microdnf update && \
    microdnf -y install ostree ostree-devel

