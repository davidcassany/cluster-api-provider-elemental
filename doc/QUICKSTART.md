# Quickstart

## Prerequisites

1. This setup uses two machines. One to deploy the CAPI management cluster and one to deploy a single node k3s cluster.  
   The machines must be able to reach each other on the network.  
   The setup assumes `192.168.122.10` will be used for the CAPI management cluster (and to expose the Elemental API).  
   `192.168.122.100` will be used by the host and used as `CONTROL_PLANE_ENDPOINT_IP` of the downstream k3s cluster.  

1. On the **management** machine, [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) is used to bootstrap a CAPI management cluster.  
   The ElementalAPI will be exposed using a `NodePort` on `30009`. This port needs to be free and not blocked by any firewall.  

1. On the **host** machine, it is recommended to disable and stop the firewall to not interfere with the k3s deployment.  

## Preparation

1. On the **management** machine, install the required dependencies: `docker`, `kind`, `helm`, `kubectl`, and `clusterctl`.
   For example on a fresh OpenSUSE Tumbleweed installation, run:

    ```bash
    # Install dependencies
    zypper install -y docker helm kubernetes1.27-client

    # Install kind
    [ $(uname -m) = x86_64 ] && curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
    chmod +x ./kind
    mv ./kind /usr/local/bin/kind

    # Install clusterctl
    curl -L https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.5.3/clusterctl-linux-amd64 -o clusterctl
    install -o root -g root -m 0755 clusterctl /usr/local/bin/clusterctl

    systemctl enable docker
    systemctl disable firewalld

    # Reboot to please docker
    reboot
    ```

1. On the **host** machine, no dependencies are needed since they will be included by `k3s`.  
   The firewall however should be disabled.  

   ```bash
   systemctl disable firewalld
   systemctl stop firewalld
   ```

## Management Cluster configuration

1. Initialize a cluster:

    ```bash
    cat << EOF | kind create cluster --name elemental-capi-management --config -
    kind: Cluster
    apiVersion: kind.x-k8s.io/v1alpha4
    nodes:
    - role: control-plane
      image: kindest/node:v1.26.6
      kubeadmConfigPatches:
      - |
        kind: InitConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "ingress-ready=true"
      extraPortMappings:
      - containerPort: 80
        hostPort: 80
        protocol: TCP
      - containerPort: 443
        hostPort: 443
        protocol: TCP
      - containerPort: 30009
        hostPort: 30009
        protocol: TCP
    EOF
    ```

1. Configure `clusterctl` to use the custom provider:

    ```bash
    mkdir -p $HOME/.cluster-api 

    cat << EOF > $HOME/.cluster-api/clusterctl.yaml
    providers:
    - name: "elemental"
      url: "https://github.com/rancher-sandbox/cluster-api-provider-elemental/releases/v0.3.0/infrastructure-components.yaml"
      type: "InfrastructureProvider"
    - name: "k3s"
      url: "https://github.com/cluster-api-provider-k3s/cluster-api-k3s/releases/v0.1.8/bootstrap-components.yaml"
      type: "BootstrapProvider"
    - name: "k3s"
      url: "https://github.com/cluster-api-provider-k3s/cluster-api-k3s/releases/v0.1.8/control-plane-components.yaml"
      type: "ControlPlaneProvider"
    EOF
    ```

1. Install CAPI Core provider, the k3s Control Plane and Bootstrap providers, and the Elemental Infrastructure provider:  

    ```bash
    ELEMENTAL_ENABLE_DEBUG="\"true\"" \
    ELEMENTAL_API_ENDPOINT="192.168.122.10.sslip.io" \
    ELEMENTAL_API_ENABLE_TLS="\"true\"" \
    ELEMENTAL_ENABLE_DEFAULT_CA="\"true\"" \
    clusterctl init --bootstrap k3s:v0.1.8 --control-plane k3s:v0.1.8 --infrastructure elemental:v0.3.0
    ```

1. Expose the Elemental API server:  

    ```bash
    cat << EOF | kubectl apply -f -
    apiVersion: v1
    kind: Service
    metadata:
      name: elemental-debug
      namespace: elemental-system
    spec:
      type: NodePort
      selector:
        control-plane: controller-manager
      ports:
      - nodePort: 30009
        port: 9090
        protocol: TCP
        targetPort: 9090    
    EOF
    ```

1. Generate `cluster.yaml` config:

    ```bash
    CONTROL_PLANE_ENDPOINT_IP=192.168.122.100 clusterctl generate cluster \
    --infrastructure elemental:v0.3.0 \
    --flavor k3s-single-node \
    --kubernetes-version v1.28.2 \
    elemental-cluster-k3s > $HOME/elemental-cluster-k3s.yaml
    ```

1. Apply `elemental-cluster-k3s.yaml` config:

    ```bash
    kubectl apply -f $HOME/elemental-cluster-k3s.yaml
    ```

1. Create a new `ElementalRegistration`:

    Note that since we are using a non-standard port for this quickstart, we are manually setting the registration `uri` field.  
    Normally this would be automatically populated by the controller from the `ELEMENTAL_API_PROTOCOL` and `ELEMENTAL_API_ENDPOINT` environment variables.  
    For more details on how to configure and expose the Elemental API, please read the related [document](./ELEMENTAL_API_SETUP.md).  

    ```bash
    cat << EOF | kubectl apply -f -
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    kind: ElementalRegistration
    metadata:
      name: my-registration
      namespace: default
    spec:
      config:
        cloudConfig:
          users:
            - name: root
              passwd: root
        elemental:
          registration:
            uri: https://192.168.122.10.sslip.io:30009/elemental/v1/namespaces/default/registrations/my-registration
          agent:
            hostname:
              useExisting: false
              prefix: "m-"
            debug: true
            osPlugin: "/usr/lib/elemental/plugins/elemental.so"
            workDir: "/oem/elemental/agent"
            postInstall:
              reboot: true
          install:
            debug: true
            device: "/dev/vda"
          reset:
            resetOem: true
            resetPersistent: true
    EOF
    ```

## (Elemental Toolkit) Host configuration

A bootable ISO image can be build using the [elemental-toolkit](https://github.com/rancher/elemental-toolkit).
The image contains the `elemental-agent` and an initial configuration to connect. Upon boot, it will auto-install an Elemental system on a machine on the configured device: `/dev/vda`.

You can configure a different device, editing the `ElementalRegistration` created above.  

- Clone this repository:

    ```bash
    git clone https://github.com/rancher-sandbox/cluster-api-provider-elemental.git
    cd cluster-api-provider-elemental
    ```

- Generate a valid agent config (depends on `kubectl` and `yq`):  

    ```bash
    ./test/scripts/print_agent_config.sh -n default -r my-registration > iso/config/my-config.yaml
    ```

  Note that the agent config should contain a valid registration `token`.  
  By default this is a JWT formatted token with no expiration.  

- Build the ISO image:

    This depends on `make` and `docker`:

    ```bash
    AGENT_CONFIG_FILE=iso/config/my-config.yaml make build-iso
    ```

- A new bootable iso should be available: `iso/elemental-dev.iso`.

## Trigger a Host reset

A Host can receive a trigger reset instruction on the following scenarios:

- The ElementalHost resource on the k8s management cluster is deleted:  

    ```bash
    kubectl delete elementalhost my-host
    ```

- The ElementalMachine associated to this host is deleted:  

    ```bash
    kubectl delete elementalmachine my-control-plane-machine
    ```

- The CAPI Cluster resource that this Host belonged to is deleted:  

    ```bash
    kubectl delete cluster my-cluster
    ```
