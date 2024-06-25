# Deployment on K3s with Ansible

The folder
[`deploy/ansible`](https://github.com/grycap/oscar/tree/master/deploy/ansible)
contains all the necessary files to deploy a [K3s](https://k3s.io/) cluster
together with the OSCAR platform using [Ansible](https://www.ansible.com/).
This way, a minified Kubernetes distribution can be used to configure OSCAR on
IoT devices located at the Edge, such as
[Raspberry PIs](https://www.raspberrypi.org/). Note that this
[playbook](https://docs.ansible.com/ansible/latest/user_guide/playbooks_intro.html)
can also be applied to quickly spread the OSCAR platform on top of any machine
or already started cloud instance since the playbook is compatible with
GNU/Linux on ARM64 and AMD64 architectures.

## Requirements

In order to use the playbook, you must install the following components:

- Ansible, following [this guide](https://docs.ansible.com/ansible/latest/installation_guide/intro_installation.html).
- The [`netaddr`](https://netaddr.readthedocs.io/en/latest/installation.html)
  python library.
- [OpenSSH](https://www.openssh.com/), to remotely access the hosts to be configured.

## Usage

### Clone the folder

First of all, you must clone the OSCAR repo:

```
git clone https://github.com/grycap/oscar.git
```

And place into the `ansible` directory:

```
cd oscar/deploy/ansible
```

### SSH configuration

As Ansible is an agentless automation tool, you must configure the
`~/.ssh/config` file for granting access to the hosts to be configured via
the SSH protocol. This playbook will use the `Host` field from SSH
configuration to set the hostnames of the nodes, so please take care of naming
them properly.

Below you can find an example of a configuration file for four nodes, being
the `front` the only one with a public IP, so it will be used as a proxy for
the SSH connection to the working nodes
([`ProxyJump`](https://www.redhat.com/sysadmin/ssh-proxy-bastion-proxyjump)
option) via its internal network.

```ssh-config
Host front
  HostName <PUBLIC_IP>
  User ubuntu
  IdentityFile ~/.ssh/my_private_key

Host wn1
  HostName <PRIVATE_IP>
  User ubuntu
  IdentityFile ~/.ssh/my_private_key
  ProxyJump front

Host wn2
  HostName <PRIVATE_IP>
  User ubuntu
  IdentityFile ~/.ssh/my_private_key
  ProxyJump front

Host wn3
  HostName <PRIVATE_IP>
  User ubuntu
  IdentityFile ~/.ssh/my_private_key
  ProxyJump front
```

### Configuration of the inventory file

Now, you have to edit the `hosts` file and add the hosts to be configured.
Note that only one node must be set in the `[front]` section, while one or
more nodes can be configured as working nodes of the cluster in the `[wn]`
section. For example, for the [previous SSH configuration](#ssh-configuration)
the `hosts` inventory file should look like this:

```ini
[front]
; Put here the frontend node as defined in .ssh/config (Host)
front

[wn]
; Put here the working nodes (one per line) as defined in the .ssh/config (Host)
wn1
wn2
wn3
```

### Setting up the playbook variables

You also need to set up some parameters for the configuration of the cluster
and OSCAR components, like OSCAR and MinIO credentials and DNS endpoints to
configure the Kubernetes Ingress and [cert-manager](https://cert-manager.io/)
to securely expose the services. To do it, please edit the `vars.yaml` file
and update the variables:

```yaml
---
# K3s version to be installed
kube_version: v1.22.3+k3s1
# Token to login in K3s and the Kubernetes Dashboard
kube_admin_token: kube-token123
# Password for OSCAR
oscar_password: oscar123
# DNS name for the OSCAR Ingress and Kubernetes Dashboard (path "/dashboard/")
dns_host: oscar-cluster.example.com
# Password for MinIO
minio_password: minio123
# DNS name for the MinIO API Ingress
minio_dns_host: minio.oscar-cluster.example.com
# DNS name for the MinIO Console Ingress
minio_dns_host_console: minio-console.oscar-cluster.example.com
```

### Installation of the required ansible roles

To install the required roles you only have to run:

```
ansible-galaxy install -r install_roles.yaml --force
```

*The `--force` argument ensures you have the latest version of the roles.*

### Running the playbook

Finally, with the following command the ansible playbook will be executed,
configuring the nodes set in the `hosts` inventory file:

```
ansible-playbook -i hosts oscar-k3s.yaml
```
