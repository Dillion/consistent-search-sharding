**Problem statement**

Imagine a product having individual spaces with embedding search. 

For example, a SaaS application might have separate workspaces for each customer, and each customer only needs to search within his/her workspace. 

Another example is a Discord-like chat application where search is within each “channel”.

How many indices should we have for such an application? (Can even be separate indexes on separate clusters)

Using 1 index for all embeddings will make the in-memory graph too large. For HNSW, size of the in-memory graph is 1.1 * (8*M + 4*dim) * 2 * (num vectors) bytes. A 32 GB RAM instance can roughly keep 1M vectors (1024 dim) in memory. 

Using 1 index for 1 workspace is too wasteful. Some customers might have very few embeddings.

Ideally we want a stable allocation of workspace ids to search indices, that minimises reindexing of workspaces when the number of search indices changes.

In standard consistent hashing, a key (workspace id) has no weight and a node has no capacity, while here a workspace id has weight proportional to the number of embeddings it contains, and a node has capacity proportional to the maximum size of the in-memory graph.

**Solution**

Use Consistent Hashing with Bounded Loads to assign each workspace to an index (node). [https://research.googleblog.com/2017/04/consistent-hashing-with-bounded-loads.html](https://research.googleblog.com/2017/04/consistent-hashing-with-bounded-loads.html)
* Ref. implementation [https://github.com/lafikl/consistent](https://github.com/lafikl/consistent) 

When a new workspace is created, assign a generated workspace id from the least loaded node.

Once we need to add/remove a node, e.g. when capacity reached, apply Consistent Hashing with Bounded Loads with the current load snapshot, and reshard the workspace ids to nodes.

**Result**

On average, when load changes with no node changes, 95% of resharding involves less than 13% of workspaces, for 5 nodes and 2000 workspaces.

On average, when a new node is added, 95% of resharding involves less than 30% of workspaces, for 3 nodes (to 4 nodes) and 1956 workspaces (avg), which is slightly less than 1/n. It follows the result of standard consistent hashing, but load aware.
  
  
![Screenshot 2024-02-15 at 3 45 21 PM](https://github.com/Dillion/consistent-search-sharding/assets/835307/133ec2da-3944-4fb0-86dd-b40885d03405)

  
**About**

This repository performs some simulations of the above solution, by generating workspace ids and corresponding load, assigning them to nodes, making load and node changes to assess the impact of resharding that occurs.
