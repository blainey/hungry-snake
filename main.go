package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
)

type Coord struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Snake struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Health int     `json:"health"`
	Body   []Coord `json:"body"`
}

type Board struct {
	Height int     `json:"height"`
	Width  int     `json:"width"`
	Food   []Coord `json:"food"`
	Snakes []Snake `json:"snakes"`
}

type Game struct {
	ID string `json:"id"`
}

type StartRequest struct {
	Game  Game  `json:"game"`
	Turn  int   `json:"turn"`
	Board Board `json:"board"`
	You   Snake `json:"you"`
}

type StartResponse struct {
	Color    string `json:"color,omitempty"`
	HeadType string `json:"headType,omitempty"`
	TailType string `json:"tailType,omitempty"`
}

type MoveRequest struct {
	Game  Game  `json:"game"`
	Turn  int   `json:"turn"`
	Board Board `json:"board"`
	You   Snake `json:"you"`
}

type MoveResponse struct {
	Move  string `json:"move"`
	Shout string `json:"shout,omitempty"`
}

type EndRequest struct {
	Game  Game  `json:"game"`
	Turn  int   `json:"turn"`
	Board Board `json:"board"`
	You   Snake `json:"you"`
}

func HandleIndex(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Your Battlesnake is alive!")
}

func HandlePing(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "pong")
}

var colorPicker uint32

type SnakeState struct {
	SelfID string
	Color  string
}

var mySnakes struct {
	sync.RWMutex
	m map[string]SnakeState
}

// HandleStart is called at the start of each game your Battlesnake is playing.
// The StartRequest object contains information about the game that's about to start.
func HandleStart(w http.ResponseWriter, r *http.Request) {
	request := StartRequest{}
	json.NewDecoder(r.Body).Decode(&request)

	var colors = []struct {
		name	string
		hexcode	string
	} {
		{ "red",	"#cc0000" },	
		{ "blue",	"#0000cc" },
		{ "green",	"#006600" },
		{ "tan",	"#996633" },
		{ "pink",	"#ff66ff" },
		{ "yellow",	"#ffff00" },
		{ "violet",	"#cc0099" },
	}  
	
	cx := atomic.AddUint32 (&colorPicker, 1) % (uint32)(len(colors))

	response := StartResponse{
		Color:    colors[cx].hexcode,
		HeadType: "bendr",
		TailType: "skinny",
	}

	var state SnakeState;
	state.SelfID = request.You.ID
	state.Color = colors[cx].name

	mySnakes.Lock()
	mySnakes.m[request.You.ID] = state
	mySnakes.Unlock()

	fmt.Print("START ID=%s, COLOR=%s\n", request.You.ID, state.Color)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Absolute value
func Abs (x int) int {
	if x < 0 {
		return -x
	} else {
		return x
	}
}

// Compute manhattan distance between two cells
func ManDist (a, b Coord) int {
	return Abs(a.X-b.X) + Abs(a.Y-b.Y)
}

// Translate a coordinate
func Translate (a Coord, dx, dy int) Coord {
	return Coord{ a.X+dx, a.Y+dy }
}

// HandleMove is called for each turn of each game.
// Valid responses are "up", "down", "left", or "right".
func HandleMove(w http.ResponseWriter, r *http.Request) {
	request := MoveRequest{}
	json.NewDecoder(r.Body).Decode(&request)

	mySnakes.RLock()
	var state = mySnakes.m[request.You.ID]
	mySnakes.RUnlock()

	color := state.Color
	fmt.Printf("MOVEREQ: COLOR=%s, %+v\n", color, request)

	// We create a local copy of the board where each cell
	// contains a value that indicates what it contains 
	//
	// Values in the cell are the following:
	// 		0		Empty
	//		1		Food
	//		3^k 	Body of snake k, k >= 1
	//		3^k+1	Head of snake k, k >= 1
	//		3^k+2	Tail of snake k, k >= 1
	//
	// k=1 is always self

	var height = request.Board.Height
	var width = request.Board.Width
	
	var grid = make ([][]int, width)
	for i := range grid {
		grid[i] = make([]int, height)
	}

	/*
	IsEmpty := func (c int) bool {
		return c == 0
	}
	*/
	IsFood := func (c int) bool {
		return c == 1
	}
	IsBody := func (c int) bool {
		return c > 2 && c%3 == 0
	}
	IsHead := func (c int) bool {
		return c > 2 && c%3 == 1
	}
	/*
	IsTail := func (c int) bool {
		return c > 2 && c%3 == 2
	}
	IsSelf := func (c int) bool {
		return c/3 == 1
	}
	SnakeNo := func (c int) int {
		return c/3
	}
	*/

	// Used to map snake number to snake ID
	var sv = make([]string,len(request.Board.Snakes)+1)
	/*
	SnakeID := func (c int) string {
		return sv[SnakeNo(c)]
	}
	*/

	// add snakes to board grid
	var ns = 2
	var myHead Coord
	for _,snake := range request.Board.Snakes {
		sx := 1
		if snake.ID != request.You.ID {
			sx = ns
			ns++
		}

		sv[sx] = snake.ID

		head := snake.Body[0]
		grid[head.X][head.Y] = 3 * sx + 1

		sz := len(snake.Body)
		for i := 1; i < sz-1; i++ {
			pos := snake.Body[i]
			grid[pos.X][pos.Y] = 3 * sx
		}

		tail := snake.Body[sz-1]
		grid[tail.X][tail.Y] = 3 * sx + 2

		if sx == 1 {
			myHead = head
		}
	}

	// add food to board
	type FoodVect struct {
		pos Coord
		dist int
	}

	var fv = make ([]FoodVect, len(request.Board.Food))

	for index,food := range request.Board.Food {
		grid[food.X][food.Y] = 1
		fv[index].pos = food
		fv[index].dist = ManDist(food,myHead)
	}

	sort.Slice(fv, func(i, j int) bool {
		return fv[i].dist < fv[j].dist
	})

	// Examine state around my snake head
	var moves = []struct { 
		label string 
		dx, dy int 
	} {
		{ "left",  -1, 	0 },
		{ "right", +1, 	0 },
		{ "up",    	0, -1 },
		{ "down",   0, +1 },
	} 
	
	type MoveOption struct {
		label string
		dist int
		risky bool
		sides int
	} 

	validMoves := make([]MoveOption,4)
	numValidMoves := 0
	for _,move := range moves {
		var c = Translate(myHead,move.dx,move.dy)

		//fmt.Printf("[COLOR=%s, Consider %s]\n", color, move.label)

		// Check if at boundary
		if c.X < 0 || c.X >= width || c.Y < 0 || c.Y >= height {
			//fmt.Printf("[COLOR=%s, Reject: boundary]\n", color);
		    continue
		}
		
		// Check if we will collide with another snake
		var cdata = grid[c.X][c.Y]
		if IsBody(cdata) || IsHead(cdata) {
			//fmt.Printf("[COLOR=%s, Reject: snake body or head]\n", color);
			continue
		}

		// Cell will be empty next turn but check if
		// we would colliude with a snake if we moved there
		mx := numValidMoves
		numValidMoves++
		validMoves[mx].label = move.label
		validMoves[mx].sides = 0
		validMoves[mx].risky = false

		for _,adj := range moves {
			ac := Translate(c,adj.dx,adj.dy)

			if ac.X == myHead.X && ac.Y == myHead.Y {
				continue
			}

			if ac.X < 0 || ac.X >= width || 
			   ac.Y < 0 || ac.Y >= height {
				validMoves[mx].sides++
				continue
			}

			adata := grid[ac.X][ac.Y]
			if IsHead(adata) {
				validMoves[mx].risky = true; 
			} else if IsBody(adata) {
				validMoves[mx].sides++
			}
		}

		if validMoves[mx].sides == 3 {
			numValidMoves--
			continue;
		}

		if IsFood(cdata) {
			validMoves[mx].dist = 0
			continue
		}

		// Compute distance to closest food
		validMoves[mx].dist = height + width
		for _,food := range fv {
			distToHere := ManDist(food.pos,myHead)
			distToNew := ManDist(food.pos,c)
			if distToNew < distToHere {
				validMoves[mx].dist = distToNew 
				//fmt.Printf("[COLOR=%s, Tentative: moves closer to food]\n", color)
				break
			}
		}
	}

	// Choose among valid moves
	var chosenMove = "left"
	if numValidMoves > 0 {
		mx := 0
		tol := width/2
		for i := 1; i < numValidMoves; i++ {
			if !validMoves[i].risky &&
			   (validMoves[i].dist == 0 || validMoves[i].dist < validMoves[mx].dist-tol ||
			    validMoves[mx].risky) {
				mx = i
			}
		}
		if mx == 0 && validMoves[0].dist > 0 {
			for i := 1; i < numValidMoves; i++ {
				if !validMoves[i].risky &&
				   (validMoves[i].sides < validMoves[mx].sides ||
					validMoves[mx].risky) {
					mx = i
				}
			}
		}
		chosenMove = validMoves[mx].label
	} 

	response := MoveResponse { chosenMove,
							   "", // shout
							 }

	fmt.Printf("MOVE: %s, COLOR=%s\n", response.Move, color)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleEnd is called when a game your Battlesnake was playing has ended.
// It's purely for informational purposes, no response required.
func HandleEnd(w http.ResponseWriter, r *http.Request) {
	request := EndRequest{}
	json.NewDecoder(r.Body).Decode(&request)

	// Nothing to respond with here
	fmt.Print("END\n")
}

func main() {
	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = "8080"
	}

	mySnakes.m = make(map[string]SnakeState)

	http.HandleFunc("/", HandleIndex)
	http.HandleFunc("/ping", HandlePing)

	http.HandleFunc("/start", HandleStart)
	http.HandleFunc("/move", HandleMove)
	http.HandleFunc("/end", HandleEnd)

	fmt.Printf("Starting Battlesnake Server at http://0.0.0.0:%s...\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
