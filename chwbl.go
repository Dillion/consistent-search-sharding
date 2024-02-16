package main

import (
	"log"
	"github.com/lafikl/consistent"
	"strconv"
	"fmt"
	"math/rand"
	"math"
	"time"
)

func calcNodeAssignment(c *consistent.Consistent, keys []string, workspaceLoad map[string]int64, workspaceMap map[string]string, update bool, suppressLog bool) int {
	count := 0
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
		if node != workspaceMap[key] {
			count += 1
			if !suppressLog {
				fmt.Printf("Workspace %s is moved from node %s to node %s\n", key, workspaceMap[key], node)
			}
		}
		if update {
			workspaceMap[key] = node
		}
	}
	return count
}

func createAndAssignNewWorkspace(c *consistent.Consistent, pregenIds [][]string, workspaceLoad map[string]int64, workspaceMap map[string]string, suppressLog bool) (string, string) {
	var currNodeToAssign string
	minLoad := int64(math.MaxInt64)
	for node, load := range c.GetLoads() {
		if load < minLoad {
			minLoad = load
			currNodeToAssign = node
		}
	}
	nodeIdx, _ := strconv.Atoi(currNodeToAssign)
	nodeIdx -= 1
	workspaceIdx := len(pregenIds[nodeIdx])-1
	newWorkspaceId := pregenIds[nodeIdx][workspaceIdx]
	pregenIds[nodeIdx] = pregenIds[nodeIdx][:workspaceIdx] // pop the last element after use
	// verify that the least loaded node is also the original assigned node in the consistent hash
	assignedNode, _ := c.Get(newWorkspaceId)
	assignedNodeConsideringLoad, _ := c.GetLeast(newWorkspaceId)
	if assignedNode != assignedNodeConsideringLoad {
		fmt.Printf("ERROR! assigned node %s and assigned node considering load %s is different", assignedNode, assignedNodeConsideringLoad)
	}
	// generate new load immediately (in real world the load will be added incrementally but we just simulate it here)
	load := rand.Int63n(2000)
	workspaceLoad[newWorkspaceId] = load
	c.UpdateLoad(currNodeToAssign, minLoad + load)
	workspaceMap[newWorkspaceId] = currNodeToAssign

	if !suppressLog {
		fmt.Printf("New node %s created, assigned to %s with load %d\n", newWorkspaceId, currNodeToAssign, load)
	}
	
	return newWorkspaceId, currNodeToAssign
}

func main() {
	rand.Seed(time.Now().UnixNano())

	var maxNodes = 5
	var maxWorkspaces = 2000
	
	// initRealm creates a consistent hash with
	// 1. keys: generated workspace ids up to maxWorkspaces count (random generation without least load rules) - this should be stored as config, or generated on the fly by a service by querying the workspace creation time
	// 2. workspaceMap: workspace ids mapped to consistent hash, considering randomly generated load - this should be stored as config
	// 3. workspaceLoad: map of randomly generated load - this can be stored as snapshot, or can be discarded
	c, keys, workspaceMap, workspaceLoad := initRealm(maxWorkspaces, maxNodes, true)
	printLoad(c, "Initial Load")

	// to check whether assignment is stable with no load change,
	// init new consistent hash and replay the creation of workspaceMap with _same_ workspaceLoad
	// note: order of workspace ids matters! (in production environment we can always retrieve this by workspace creation time)
	c = newConsistentHash(maxNodes)
	calcNodeAssignment(c, keys, workspaceLoad, workspaceMap, false, false)
	printLoad(c, "Replayed Load")

	fmt.Println()
	fmt.Printf("Now simulating single load snapshot change ...")
	// to check how assignment changes after load is modified randomly,
	// init new consistent hash, replay the creation of workspaceMap with _modified_ workspaceLoad
	c = newConsistentHash(maxNodes)
	// simulate random load change of -20 (removing documents) to 500 (adding documents)
	var minLoadChange int64 = -50
	var maxLoadChange int64 = 500
	modifyLoad(workspaceLoad, minLoadChange, maxLoadChange)
	changes := calcNodeAssignment(c, keys, workspaceLoad, workspaceMap, false, true)
	fmt.Printf("Single load update across %d nodes and %d workspaces makes %d reshardings\n", maxNodes, maxWorkspaces, changes)

	// get stats of numUpdates load modifications
	numUpdates := 10000
	fmt.Println()
	fmt.Printf("Now simulating %d load snapshot changes ...\n", numUpdates)
	var allChanges []int
	for i := 0; i < numUpdates; i++ {
		c, keys, workspaceMap, workspaceLoad := initRealm(maxWorkspaces, maxNodes, true)
		c = newConsistentHash(maxNodes)
		// randomize the change in load also
		var minLoadChange int64 = -200 + rand.Int63n(201) // in the range [-200, 0]
		var maxLoadChange int64 = 1 + rand.Int63n(2001) // in the range [1, 2000]
		modifyLoad(workspaceLoad, minLoadChange, maxLoadChange)
		changes := calcNodeAssignment(c, keys, workspaceLoad, workspaceMap, false, true)
		allChanges = append(allChanges, changes)
	}
	min, max, avg, stdDev := calculateStats(allChanges)
	msg := fmt.Sprintf("%d updates across %d nodes, %d workspaces", numUpdates, maxNodes, maxWorkspaces)
	log.Println(msg)
	fmt.Printf("Min: %d\n", min)
	fmt.Printf("Max: %d\n", max)
	fmt.Printf("Average: %.2f\n", avg)
	fmt.Printf("Standard Deviation: %.2f\n", stdDev)
	// Recorded values for 5 nodes and 200 workspaces:
	// Average: 26.35
	// Standard Deviation: 9.18
	// 95% of changes are below 44.71
	// stable across random workspace id generation, random initial load, and random load modifications!!
	// Recorded values for 5 nodes and 2000 workspaces:
	// Average: 181.96
	// Standard Deviation: 35.22
	// 95% of changes are below 252.4
	// As num workspaces increase, less resharding occurs!!

	// now lets benchmark change in nodes and workspaces
	// Assume:
	// 1024 dim vectors, 16 links per element (HNSW parameter), 2 replicas
	// Size of inmemory graph = 1.1 * (8*M + 4*dim) * 2 * num vectors (bytes) = 9139 bytes per vector
	// m5.2xlarge has 32GB RAM. 50% is allocated for the JVM heap, and 50% of the remainder is the default circuit breaker limit = max size of inmemory graph is 8GB
	// 8GB / 9139 bytes = 939920 vectors in memory for m5.2xlarge
	// if workspace has on average 1000 vectors, 940 workspaces can fit on 1 index
	// if we set 70% as threshold for adding index, 
	// > 657944 vectors will trigger adding a new node

	// Starting configuration:
	// 3 nodes (indices), 1000 workspaces
	fmt.Println()
	fmt.Printf("Now simulating workspace additions until capacity threshold reached, then add a single node and check number of node assignments changed ...\n")
	maxNodes = 3
	maxWorkspaces = 1500
	c, keys, workspaceMap, workspaceLoad = initRealm(maxWorkspaces, maxNodes, true)
	printLoad(c, "Starting Load")

	// simulate a key generator that creates random ids and calculates each of their hash, to queue and directly distribute later
	pregenIds := initKeyGen(c, maxNodes)

	// now the key generator can distribute keys on request whenever a new workspace is added
	// first check the current load of all nodes, and select the least loaded node
	// get the first key (last element in ds) for that node, and return it for the new workspace
	// note that because we already check for current load, the consistent hash will always match the original value, i.e. c.Get and c.GetLeast will be the same for this new workspace
	// we can verify that when we add the new workspace to the consistent hash
	addWorkspace := -1
	for addWorkspace < 0 {
		_, currNodeToAssign := createAndAssignNewWorkspace(c, pregenIds, workspaceLoad, workspaceMap, true)
		if loadAboveThreshold(c, currNodeToAssign, 657944) {
			printLoad(c, "Load exceeded, starting new node addition")
			maxNodes += 1
			// now calculate the new node assignment and see how many changed
			c = newConsistentHash(maxNodes)
			changes := calcNodeAssignment(c, keys, workspaceLoad, workspaceMap, true, true)
			fmt.Printf("%d node assignments changed\n", changes)
			printLoad(c, "Updated load")
			break
		}
	}

	// run multiple simulations and get stats
	numUpdates = 10000
	fmt.Println()
	fmt.Printf("Now running %d simulations of previous scenario ...\n", numUpdates)
	allChanges = []int{}
	var additionsCount []int
	for i := 0; i < numUpdates; i++ {
		maxNodes = 3
		maxWorkspaces = 1500
		c, keys, workspaceMap, workspaceLoad := initRealm(maxWorkspaces, maxNodes, true)
		pregenIds := initKeyGen(c, maxNodes)
		maxAdditions := math.MaxInt64
		workspaceAdditions := 0
		for workspaceAdditions < maxAdditions {
			_, currNodeToAssign := createAndAssignNewWorkspace(c, pregenIds, workspaceLoad, workspaceMap, true)
			if loadAboveThreshold(c, currNodeToAssign, 657944) {
				maxNodes += 1
				c = newConsistentHash(maxNodes)
				changes := calcNodeAssignment(c, keys, workspaceLoad, workspaceMap, true, true)
				allChanges = append(allChanges, changes)
				additionsCount = append(additionsCount, workspaceAdditions)
				break
			}
			workspaceAdditions += 1
		}
	}
	min, max, avg, stdDev = calculateStats(allChanges)
	_, _, avgAdditions, _ := calculateStats(additionsCount)
	msg = fmt.Sprintf("Workspaces resharded when new node added, after average of %f additions with starting %d workspaces", avgAdditions, maxWorkspaces)
	log.Println(msg)
	fmt.Printf("Min: %d\n", min)
	fmt.Printf("Max: %d\n", max)
	fmt.Printf("Average: %.2f\n", avg)
	fmt.Printf("Standard Deviation: %.2f\n", stdDev)
}