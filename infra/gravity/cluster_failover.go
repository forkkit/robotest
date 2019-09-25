/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package gravity

import (
	"context"

	"github.com/gravitational/robotest/lib/utils"
	"github.com/gravitational/robotest/lib/wait"
	"github.com/gravitational/trace"
)

// Failover isolates the current leader node and elects a new leader node.
// Conforms to ConfigFn interface.
func (c *TestContext) Failover(cluster []Gravity) error {
	// TODO: Configure timeouts
	ctx, cancel := context.WithTimeout(c.ctx, c.timeouts.Status)
	defer cancel()

	c.Logger().WithField("cluster", cluster).Info("Start failover test")

	// Get initial leader node

	oldLeader, err := getLeaderNode(ctx, cluster)
	if err != nil {
		c.Logger().WithError(err).Error("Failed to get initial leader")
		return trace.Wrap(err)
	}
	c.Logger().WithField("leader", oldLeader).Info("Initial leader node")

	// Create network partition

	if err := oldLeader.PartitionNetwork(ctx, cluster); err != nil {
		c.Logger().WithError(err).Error("Failed to created network partition")
		return trace.Wrap(err, "failed to create network partition")
	}
	partitions := getPartitions(cluster, oldLeader)
	c.Logger().WithField("partitions", partitions).Info("Created network partition")

	// Wait for new leader election

	c.Logger().Info("Waiting for new leader to be elected")
	retry := wait.Retryer{
		Attempts: leaderElectionRetries,
		Delay:    leaderElectionWait,
	}
	if err = retry.Do(ctx, retryNewLeaderElected(ctx, partitions[1])); err != nil {
		c.Logger().WithError(err).Error("Failed to elect new leader")
		return trace.Wrap(err, "new leader was not elected")
	}
	newLeader, err := getLeaderNode(ctx, partitions[1])
	if err != nil {
		c.Logger().WithError(err).Error("Failed to get new leader")
		return trace.Wrap(err)
	}
	c.Logger().WithField("leader", newLeader).Info("New leader elected")

	// Verify cluster is still functional

	if err := c.Status(partitions[1]); err != nil {
		c.Logger().WithError(err).Error("Cluster partition is nonoperational")
		return trace.Wrap(err, "cluster partition is nonoperational")
	}
	c.Logger().WithField("cluster", partitions[1]).Info("Cluster is functional")

	// Remove network partition

	if err := oldLeader.UnpartitionNetwork(ctx, cluster); err != nil {
		c.Logger().WithError(err).Error("Failed to remove network partition")
		return trace.Wrap(err, "failed to remove network partition")
	}
	c.Logger().WithField("cluster", cluster).Info("Removed network partition")

	// Verify healthy cluster status

	retry = wait.Retryer{
		Attempts: activeStatusRetries,
		Delay:    activeStatusWait,
	}
	err = retry.Do(ctx, retryClusterIsActive(ctx, cluster))
	if err != nil {
		c.Logger().WithError(err).Error("Cluster has not recovered healthy status")
		return trace.Wrap(err, "cluster has not recovered healthy status")
	}
	c.Logger().Info("Cluster status is active")

	return nil
}

// retryNewLeaderElected returns a retry function. This function verifies that a
// new leader has been elected.
func retryNewLeaderElected(ctx context.Context, cluster []Gravity) (retryFunc func() error) {
	return func() error {
		_, err := getLeaderNode(ctx, cluster)
		if err != nil {
			return wait.Continue("new leader not yet elected: %v", err)
		}
		return nil
	}
}

// retryClusterIsActive returns a retry function. This function verifies that
// the cluster status is active.
func retryClusterIsActive(ctx context.Context, cluster []Gravity) (retryFunc func() error) {
	return func() error {
		statusChan := make(chan interface{}, len(cluster))
		errChan := make(chan error, len(cluster))

		for _, node := range cluster {
			go func(n Gravity) {
				status, err := n.Status(ctx)
				errChan <- err
				statusChan <- status
			}(node)
		}
		statuses, err := utils.Collect(ctx, nil, errChan, statusChan)
		if err != nil {
			return wait.Continue("status not available on some nodes: %v", err)
		}
		for _, s := range statuses {
			status, ok := s.(*GravityStatus)
			if !ok {
				return trace.BadParameter("did not return GravityStatus: %v", s)
			}
			if status.Cluster.Status != StatusActive {
				return wait.Continue("cluster status is not active: %v", status)
			}
		}
		return nil
	}
}

// getLeaderNode returns the current leader node.
// If there are multiple nodes that think they are the leader, this function
// will return the first one it encounters.
func getLeaderNode(ctx context.Context, cluster []Gravity) (leader Gravity, err error) {
	for _, node := range cluster {
		if node.IsLeader(ctx) {
			return node, nil
		}
	}
	return nil, trace.NotFound("this cluster does not contain a leader node")
}

// getPartitions returns the two network partitions created when isolating
// node from the cluster.
func getPartitions(cluster []Gravity, node Gravity) (partitions [2][]Gravity) {
	partitions[0] = []Gravity{node}
	for i, n := range cluster {
		if n == node {
			partitions[1] = append(partitions[1], cluster[:i]...)
			partitions[1] = append(partitions[1], cluster[i+1:]...)
			break
		}
	}
	return partitions
}
