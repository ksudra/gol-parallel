package gol

import (
	//"flag"
	//"fmt"
	//"net/rpc"
	"strconv"
	"strings"
	"sync"
	// "os"
	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

type GameOfLife struct {
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	world := buildWorld(p, c)
	//alive := make(chan []util.Cell)
	//ticker := time.NewTicker(2 * time.Second)

	// TODO: Create a 2D slice to store the world.

	turn := 0

	for turn = 0; turn < p.Turns; turn++ {
		//go func() {
		//	for range ticker.C {
		//		alive := len(calculateAliveCells(world))
		//		c.events <- AliveCellsCount{
		//			CompletedTurns: turn,
		//			CellsCount:     alive,
		//		}
		//	}
		//}()

		world = calculateNextState(p, world)
		c.events <- TurnComplete{
			CompletedTurns: turn,
		}

	}

	// TODO: Execute all turns of the Game of Life.
	sendWorld(p, c, world, turn)

	c.events <- FinalTurnComplete{
		CompletedTurns: turn,
		Alive:          calculateAliveCells(world),
	}

	name := strings.Join([]string{strconv.Itoa(p.ImageWidth), strconv.Itoa(p.ImageHeight), strconv.Itoa(turn)}, "x")

	c.events <- ImageOutputComplete{
		CompletedTurns: turn,
		Filename:       strings.Join([]string{name, strconv.Itoa(p.Threads)}, "-"),
	}

	// TODO: Report the final state using FinalTurnCompleteEvent.

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

func buildWorld(p Params, c distributorChannels) [][]byte {
	c.ioCommand <- ioInput
	c.ioFilename <- strings.Join([]string{strconv.Itoa(p.ImageWidth), strconv.Itoa(p.ImageHeight)}, "x")

	world := make([][]byte, p.ImageHeight)
	for y := range world {
		world[y] = make([]byte, p.ImageWidth)
		for x := range world[y] {
			world[y][x] = <-c.ioInput
		}
	}

	return world
}
func sendWorld(p Params, c distributorChannels, world [][]byte, turn int) {
	c.ioCommand <- ioOutput
	name := strings.Join([]string{strconv.Itoa(p.ImageWidth), strconv.Itoa(p.ImageHeight), strconv.Itoa(turn)}, "x")
	c.ioFilename <- strings.Join([]string{name, strconv.Itoa(p.Threads)}, "-")
	for y := range world {
		for x := range world[y] {
			c.ioOutput <- world[y][x]
		}
	}
}

func countNeighbours(p Params, x int, y int, world [][]uint8) int {
	neighbours := [8][2]int{
		{-1, -1},
		{-1, 0},
		{-1, 1},
		{0, -1},
		{0, 1},
		{1, -1},
		{1, 0},
		{1, 1},
	}

	count := 0

	for _, r := range neighbours {
		if world[(y+r[0]+p.ImageHeight)%p.ImageHeight][(x+r[1]+p.ImageWidth)%p.ImageWidth] == 255 {
			count++
		}
	}

	return count
}

func calculateNextState(p Params, world [][]byte) [][]byte {
	tempWorld := make([][]byte, len(world))
	for i := range world {
		tempWorld[i] = make([]byte, len(world[i]))
		copy(tempWorld[i], world[i])
	}

	var wg sync.WaitGroup
	var remainder sync.WaitGroup

	for i := 0; i < p.Threads; i++ {
		start := i * (p.ImageHeight - p.ImageHeight%p.Threads) / p.Threads
		end := start + (p.ImageHeight-p.ImageHeight%p.Threads)/p.Threads
		wg.Add(1)
		go worker(&wg, start, end, p, tempWorld, world)

	}
	wg.Wait()

	if p.ImageHeight%p.Threads > 0 {
		start := p.ImageHeight - p.ImageHeight%p.Threads
		remainder.Add(1)
		go worker(&remainder, start, p.ImageHeight, p, tempWorld, world)
	}

	remainder.Wait()

	return tempWorld
}

func worker(wg *sync.WaitGroup, start int, end int, p Params, newWorld [][]byte, world [][]byte) {
	defer wg.Done()

	for y := start; y < end; y++ {
		for x := range newWorld {
			count := countNeighbours(p, x, y, world)

			if world[y][x] == 255 && (count < 2 || count > 3) {
				newWorld[y][x] = 0
			} else if world[y][x] == 0 && count == 3 {
				newWorld[y][x] = 255
			}
		}
	}
}

func calculateAliveCells(world [][]byte) []util.Cell {
	var cells []util.Cell
	for i := range world {
		for j := range world[i] {
			if world[i][j] == 255 {
				cells = append(cells, util.Cell{X: j, Y: i})
			}
		}
	}
	return cells
}
