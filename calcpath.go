package main

import "math/rand"

type TNodeState struct {
	isMarked int
	fromNode int
}

const (
	MaxNumPaths = 50
	MaxPathLen  = 7
)

var (
	paths    [MaxNumPaths][MaxPathLen]int
	numPaths int

	nodeStates [NumOfNodes]TNodeState
	dstNode    int
	pathLen    int
)

func calcPathRecurse(prevNode, curNode int) {
	if curNode == dstNode {

		// One possible path found, let's add it to the list
		for i := pathLen - 1; i >= 0; i-- {
			paths[numPaths][i] = curNode
			curNode = nodeStates[curNode].fromNode
		}

		paths[numPaths][pathLen] = UndefNode
		numPaths++
	} else {
		// Call recursively each node we can reach from our current position
		for nextNode := 0; nextNode < NumOfNodes; nextNode++ {
			if walkMatrix[prevNode][curNode][nextNode] != 0 && nodeStates[nextNode].isMarked == 0 {
				nodeStates[nextNode].isMarked = 1
				nodeStates[nextNode].fromNode = curNode
				pathLen++
				calcPathRecurse(curNode, nextNode)
				nodeStates[nextNode].isMarked = 0
				pathLen--
			}
		}
	}
}

func calcPath(fromNode, toNode int) *int {
	// Note: this is certainly not the exact algorithm used in the original,
	// but so far it is the best I could imagine to fit the need.

	var res *int = nil

	for i := 0; i < NumOfNodes; i++ {
		nodeStates[i].isMarked = 0
		nodeStates[i].fromNode = 0
	}

	dstNode = toNode
	numPaths = 0
	pathLen = 1
	nodeStates[fromNode].isMarked = 1
	nodeStates[fromNode].fromNode = UndefNode

	calcPathRecurse(UndefNode, fromNode)

	//if (debugMode) {
	//
	//	printf("\n +-- CALCULATE PATH --\n |\n");
	//	printf(" |  . walking from %c to %c:\n", 'A' + fromNode, 'A' + toNode);
	//	printf(" |  . possible paths: ");
	//
	//	for (int j=0; j < numPaths; j++) {
	//		putchar(' ');
	//		for (int i=0; paths[j][i] != UNDEF_NODE; i++)
	//		printf("%c", 'A' + paths[j][i]);
	//	}
	//
	//	putchar('\n');
	//}

	// NOTE: original C code, took a pointer to the 0th int
	// in the subarray since in C arrays decay into pointers
	// This should be equivalent.
	res = &paths[rand.Int()%numPaths][0]
	//if (debugMode) {
	//
	//	printf(" |  . chosen path: ");
	//
	//	for (int i=0; res[i] != UNDEF_NODE; i++)
	//	printf("%c", 'A' + res[i]);
	//
	//	printf("\n +--------------------\n\n");
	//}

	return res
}
