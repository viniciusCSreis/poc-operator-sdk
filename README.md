# poc-operator-sdk

Install operator-sdk:

Para usuarios mac ou linux:
https://sdk.operatorframework.io/docs/installation/

Para usuarios windows:
fazer o build do repo: https://github.com/operator-framework/operator-sdk

```
mkdir memcached-operator
cd memcached-operator
operator-sdk init --domain example.com --repo github.com/example/memcached-operator
```

```
operator-sdk create api --group cache --version v1alpha1 --kind Memcached --resource --controller
```

```
// MemcachedSpec defines the desired state of Memcached
type MemcachedSpec struct {
//+kubebuilder:validation:Minimum=0
// Size is the size of the memcached deployment
Size int32 `json:"size"`
}

// MemcachedStatus defines the observed state of Memcached
type MemcachedStatus struct {
// Nodes are the names of the memcached pods
Nodes []string `json:"nodes"`
}
```

```
make generate
make manifests
make install
make run
```