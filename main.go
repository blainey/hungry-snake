package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	//"sync"
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

/*
type GameState struct {
	SelfID string
	Heads, Tails map[string]Coord
}

var games struct {
	sync.RWMutex
	m map[string]GameState
}
*/

func HandleIndex(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Your Battlesnake is alive!")
}

func HandlePing(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "pong")
}

// HandleStart is called at the start of each game your Battlesnake is playing.
// The StartRequest object contains information about the game that's about to start.
// TODO: Use this function to decide how your Battlesnake is going to look on the board.
func HandleStart(w http.ResponseWriter, r *http.Request) {
	request := StartRequest{}
	json.NewDecoder(r.Body).Decode(&request)

/*
        NOTE: this will not work when this snake is added multiple times to a game
        Also game state not currently used so fix this if/when needed

	var state GameState;
	state.SelfID = request.You.ID
	state.Heads = make(map[string]Coord)
	state.Tails = make(map[string]Coord)

	// store snake head/tail locations
	for _,snake := range request.Board.Snakes {
		state.Heads[snake.ID] = snake.Body[0]
		state.Tails[snake.ID] = snake.Body[0]
	}

	games.Lock()
	games.m[request.Game.ID] = state
	games.Unlock()
*/

	response := StartResponse{
		Color:    "#BF260A",
		HeadType: "bendr",
		TailType: "skinny",
	}

	fmt.Print("START ID=%s\n", request.You.ID)
	
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
// TODO: Use the information in the MoveRequest object to determine your next move.
func HandleMove(w http.ResponseWriter, r *http.Request) {
	request := MoveRequest{}
	json.NewDecoder(r.Body).Decode(&request)

	fmt.Printf("MOVEREQ: %+v\n", request)

/*
	games.RLock()
	var state = games.m[request.Game.ID]
	games.RUnlock()
*/

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
		//state.Heads[snake.ID] = head

		sz := len(snake.Body)
		for i := 1; i < sz-1; i++ {
			pos := snake.Body[i]
			grid[pos.X][pos.Y] = 3 * sx
		}

		tail := snake.Body[sz-1]
		grid[tail.X][tail.Y] = 3 * sx + 2
		//state.Tails[snake.ID] = tail

		if sx == 1 {
			myHead = head
		}
	}

/*
	games.Lock()
	games.m[request.Game.ID] = state
	games.Unlock()
*/

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
	
	var chosenMove = "none"
	var distToClosest = height + width
	var riskyMove = "none"
	for _,move := range moves {
		var c = Translate(myHead,move.dx,move.dy)

		//fmt.Printf("[Consider %s]\n", move.label)

		// Check if at boundary
		if c.X < 0 || c.X >= width || c.Y < 0 || c.Y >= height {
			//fmt.Printf("[Reject: boundary]\n");
		    continue
		}
		
		// Check if we will collide with another snake
		var cdata = grid[c.X][c.Y]
		if IsBody(cdata) || IsHead(cdata) {
			//fmt.Printf("[Reject: snake body or head]\n");
			continue
		}

		// Cell will be empty next turn but check if
		// we would colliude with a snake if we moved there
		var collide = false
		for _,adj := range moves {
			ac := Translate(c,adj.dx,adj.dy)
			if ac.X < 0 || ac.X >= width || 
			   ac.Y < 0 || ac.Y >= height {
				continue
			}
			if ac.X == myHead.X && ac.Y == myHead.Y {
				continue
			}
			if IsHead(grid[ac.X][ac.Y]) {
				collide = true; 
				break
			}
		}

		if collide { 
			//fmt.Printf("[Reject: would collide with other snake head]\n")
                        riskyMove = move.label
			continue 
		}

		if IsFood(cdata) {
			//fmt.Printf("[Choose: contains food]\n")
			chosenMove = move.label
			break
		}

		// Cell will be empty next turn, so choose it if 
		// its the only option, or if it moves us closer to 
		// the closest food
		for _,food := range fv {
			distToHere := ManDist(food.pos,myHead)
			if (distToHere > distToClosest) { break }

			distToNew := ManDist(food.pos,c)
			if distToNew < distToHere && 
			   distToNew < distToClosest {
				distToClosest = distToNew
				chosenMove = move.label
				//fmt.Printf("[Tentative: moves closer to food]\n")
				break
			}
		}

		if chosenMove == "none" {
			//fmt.Printf("[Tentative: default]\n")
			chosenMove = move.label
		}
	}

	// if no good move, any will do ...
	if chosenMove == "none" {
		if riskyMove != "none" {
			//fmt.Printf("[Choose: %s, risky]\n")
			chosenMove = riskyMove
		} else {
			//fmt.Printf("[Choose: left, suicide]\n")
			chosenMove = "left"
		}
	}

	// Choose a random direction to move in
	//possibleMoves := []string{"up", "down", "left", "right"}
	//move := possibleMoves[rand.Intn(len(possibleMoves))]

	response := MoveResponse { chosenMove,
							   "", // shout
							 }

	fmt.Printf("MOVE: %s\n", response.Move)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleEnd is called when a game your Battlesnake was playing has ended.
// It's purely for informational purposes, no response required.
func HandleEnd(w http.ResponseWriter, r *http.Request) {
	request := EndRequest{}
	json.NewDecoder(r.Body).Decode(&request)

/*
	games.Lock()
	delete(games.m, request.Game.ID)
	games.Unlock()
*/

	// Nothing to respond with here
	fmt.Print("END\n")
}

func main() {
	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = "8080"
	}

	//games.m = make(map[string]GameState)

	http.HandleFunc("/", HandleIndex)
	http.HandleFunc("/ping", HandlePing)

	http.HandleFunc("/start", HandleStart)
	http.HandleFunc("/move", HandleMove)
	http.HandleFunc("/end", HandleEnd)

	fmt.Printf("Starting Battlesnake Server at http://0.0.0.0:%s...\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
