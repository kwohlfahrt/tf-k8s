package provider

import (
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func loadConfig(kubeconfig []byte) (*rest.Config, error) {
	clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeconfig)
	if err != nil {
		return nil, err
	}
	return clientConfig.ClientConfig()
}

func MakeDynamicClient(kubeconfig []byte) (*dynamic.DynamicClient, error) {
	restConfig, err := loadConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	client, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func MakeDiscoveryClient(kubeconfig []byte) (*discovery.DiscoveryClient, error) {
	restConfig, err := loadConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	return discovery.NewDiscoveryClientForConfig(restConfig)
}
