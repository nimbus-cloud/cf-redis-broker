package redis

import (
	"github.com/pivotal-cf/cf-redis-broker/brokerconfig"
	"github.com/pivotal-cf/cf-redis-broker/broker"
)

var _ = broker.InstanceCreator(&ReplicatedInstanceCreator{})

type ReplicatedInstanceCreator struct {
	localCreator *LocalInstanceCreator
	config    	 brokerconfig.Config
	slaveBrokerClient *SlaveBrokerClient
}

func NewReplicatedInstanceCreator(localCreator *LocalInstanceCreator, config brokerconfig.Config) broker.InstanceCreator {
	slaveBrokerClient := NewSlaveBrokerClient(config.RedisConfiguration.OtherSideIP, config.RedisConfiguration.BrokerPort,
		config.AuthConfiguration.Username, config.AuthConfiguration.Password)

	replicatedInstanceCreator := &ReplicatedInstanceCreator{
		localCreator: localCreator,
		config:       config,
		slaveBrokerClient: slaveBrokerClient,
	}

	return replicatedInstanceCreator
}

func (replicatedCreator *ReplicatedInstanceCreator) Create(instanceID string) error {

	err := replicatedCreator.localCreator.Create(instanceID)
	if err != nil {
		return err
	}

	if replicatedCreator.config.RedisConfiguration.Master {
		instance, err := replicatedCreator.localCreator.FindByID(instanceID)
		if err != nil {
			return err
		}

		err = replicatedCreator.slaveBrokerClient.CreateSlaveInstance(instance)
		if err != nil {
			return err
		}
	}

	return nil
}

func (replicatedCreator *ReplicatedInstanceCreator) Destroy(instanceID string) error {
	if replicatedCreator.config.RedisConfiguration.Master {
		instance, err := replicatedCreator.localCreator.FindByID(instanceID)
		if err != nil {
			return err
		}
		err = replicatedCreator.slaveBrokerClient.DestroySlaveInstance(instance)
		if err != nil {
			return err
		}
	}
	return replicatedCreator.localCreator.Destroy(instanceID)
}

func (replicatedCreator *ReplicatedInstanceCreator) InstanceExists(instanceID string) (bool, error) {
	return replicatedCreator.localCreator.InstanceExists(instanceID)
}


