package main

import (
	"log"
	"github.com/lafikl/consistent"
	"github.com/google/uuid"
	"strconv"
	"fmt"
)

func newConsistentHash(maxNodes int) *consistent.Consistent {
	c := consistent.New()
	for i := 1; i <= maxNodes; i++ {
		c.Add(strconv.Itoa(i))
	}
	return c
}

// simulate key generator that queues workspacesIds for each node
func initKeyGen(c *consistent.Consistent, maxNodes int) [][]string {
	pregenIds := make([][]string, maxNodes)
	for i := range pregenIds {
		pregenIds[i] = make([]string, 0)
	}
	for i := 0; i < 10000; i++ {
		newUUID := uuid.New()
		workspaceId := newUUID.String()
		// queue all ids by their original hashed node
		node, err := c.Get(workspaceId)
		if err != nil {
			log.Fatal(err)
		}
		nodeId, _ := strconv.Atoi(node)
		nodeId -= 1
		pregenIds[nodeId] = append(pregenIds[nodeId], workspaceId)
	}
	return pregenIds
}

func initRealm(maxWorkspaces int, maxNodes int, suppressLog bool) (*consistent.Consistent, []string, map[string]string, map[string]int64) {
	c := newConsistentHash(maxNodes)

	var keys []string
	workspaceMap := make(map[string]string) // map of workspaceId to node
	workspaceLoad := make(map[string]int64) // map of workspaceId to load
	
	// build initial workspaceLoad map
	for i := 0; i < maxWorkspaces; i++ {
		newUUID := uuid.New()
		workspaceId := newUUID.String()
		workspaceLoad[workspaceId] = 0
		keys = append(keys, workspaceId)
	}
	setLoad(workspaceLoad, 2000)

	// calculate hashed position from load snapshot
	for _, key := range keys {
		node, err := c.GetLeast(key)
		if err != nil {
			log.Fatal(err)
		}

		var currLoad int64
		for n, load := range c.GetLoads() {
			if n == node {
				currLoad = load
			}
		}

		c.UpdateLoad(node, currLoad + workspaceLoad[key])

		if !suppressLog {
			// check the node assignment and load update one by one
			originalNode, _ := c.Get(key)
			fmt.Printf("Adding workspace %s with load %d to node %s, original designated node %s\n", key, workspaceLoad[key], node, originalNode)
			// (total_load/number_of_hosts)*1.25
			fmt.Printf("MaxLoad %d\n", c.MaxLoad())
			for n, load := range c.GetLoads() {
				fmt.Printf("%s: %d\n", n, load)
			}
		}
		workspaceMap[key] = node
	}
	return c, keys, workspaceMap, workspaceLoad
}