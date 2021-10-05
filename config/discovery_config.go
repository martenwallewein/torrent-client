package config

type PeerDiscoveryConfig struct {
	EnableDht     bool // start dht node
	EnableTracker bool // TODO: implementation currently doesnt support SCION-trackers
}

// DefaultPeerDisoveryConfig use all supported dynamic peer discovery techniques
func DefaultPeerDisoveryConfig() PeerDiscoveryConfig {
	return PeerDiscoveryConfig{
		EnableDht:     true,
		EnableTracker: false,
	}
}

// NoPeerDisoveryConfig don't use any type of dynamic peer discovery technique
func NoPeerDisoveryConfig() PeerDiscoveryConfig {
	return PeerDiscoveryConfig{
		EnableDht:     false,
		EnableTracker: false,
	}
}
