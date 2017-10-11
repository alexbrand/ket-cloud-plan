# KET CloudPlan

## Overview
This tool allows you to add nodes to your KET plan file based on cloud-provider
metadata that is attached to your infrastructure. Use your favorite infrastructure
provisioning tool to create and tag your machines.

## Supported Cloud Providers
- [x] AWS EC2
- [ ] Microsoft Azure 
- [ ] Google Cloud Platform
- [ ] Packet

## Quickstart

1. Assign roles to each of your nodes using the `KetRole` tag.
1. Profit:
```
# ./kcp aws --region us-east-1 --role-tag KetRole
etcd:
- host: ip-10-0-3-132.ec2.internal
  ip: 54.145.139.39
  internalip: 10.0.3.132
master:
- host: ip-10-0-3-226.ec2.internal
  ip: 34.207.127.13
  internalip: 10.0.3.226
worker:
- host: ip-10-0-3-215.ec2.internal
  ip: 54.152.221.54
  internalip: 10.0.3.215
```
