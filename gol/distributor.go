package gol

import (
	"bufio"
	"flag"
	"fmt"
	"net/rpc"
	"os"
	"strconv"
	"sync"
	"uk.ac.bris.cs/gameoflife/stubs"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

var globalWorld [][]byte
var globalP Params
var turns int
var mutex = &sync.Mutex{}

func createWorld() [][]byte {
	world := make([][]byte, globalP.ImageHeight)
	for i := range world {
		world[i] = make([]byte, globalP.ImageWidth)
	}
	return world
}

func getInput(c distributorChannels) {
	for i := range globalWorld {
		for j := range globalWorld[i] {
			globalWorld[i][j] = <-c.ioInput
		}
	}
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	// TODO: Create a 2D slice to store the world.
	globalP = p
	globalWorld = createWorld()
	turn := 0

	c.ioCommand <- ioInput
	string := strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.ImageWidth)
	c.ioFilename <- string

	getInput(c)

	// fixing rn
	server := flag.String("server", "127.0.0.1:8030", "IP:port string to connect to as server")
	flag.Parse()
	fmt.Println("Server: ", *server)
	client, _ := rpc.Dial("tcp", *server)
	defer client.Close()
	for {
		request := stubs.Request{World: globalWorld}
		response := new(stubs.Response)
		client.Call(stubs.PremiumReverseHandler, request, response)
		fmt.Println("Responded: " + response.Message)
	}

	for i := 0; i < p.Turns; i++ {
		globalWorld = calculateNextState(p, globalWorld, 0, p.ImageHeight, 0, p.ImageWidth)
	}

	event := FinalTurnComplete{CompletedTurns: p.Turns, Alive: calculateAliveCells(p, globalWorld)}
	c.events <- event

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
