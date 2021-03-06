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
	"time"
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

	fmt.Print("START: COLOR=%s\n", state.Color)
	
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
	start := time.Now()

	request := MoveRequest{}
	json.NewDecoder(r.Body).Decode(&request)

	mySnakes.RLock()
	var state = mySnakes.m[request.You.ID]
	mySnakes.RUnlock()

	color := state.Color
	fmt.Printf("MOVE: COLOR=%s, Turn=%d\n", color, request.Turn)

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
	IsTail := func (c int) bool {
		return c > 2 && c%3 == 2
	}
	IsSelf := func (c int) bool {
		return c/3 == 1
	}
	IsSpace := func (c int) bool {
		return c < 3
	}
	/*
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
		trap bool
		region int
		corner bool
	} 

	IsCorner := func(s Coord) bool {
		return (s.X == 0 || s.X == width-1) && (s.Y == 0 || s.Y == height-1)
	}

	vm := make([]MoveOption,4)
	numvm := 0
	for _,move := range moves {
		var c = Translate(myHead,move.dx,move.dy)

		fmt.Printf("[COLOR=%s, Consider %s]\n", color, move.label);

		// Check if at boundary
		if c.X < 0 || c.X >= width || c.Y < 0 || c.Y >= height {
			fmt.Printf("[COLOR=%s, Reject %s: boundary]\n", color, move.label);
		    continue
		}
		
		// Check if we will collide with another snake
		var cdata = grid[c.X][c.Y]
		if IsBody(cdata) || IsHead(cdata) ||
		   ( IsSelf(cdata) && IsTail(cdata) ) {
			fmt.Printf("[COLOR=%s, Reject %s: snake body or head]\n", color, move.label);
			continue
		}

		// Cell will be empty next turn but check if
		// we would colliude with a snake if we moved there
		mx := numvm
		numvm++
		vm[mx].label = move.label
		vm[mx].sides = 0
		vm[mx].risky = IsTail(cdata)
		vm[mx].trap = false
		vm[mx].region = 0
		vm[mx].corner = IsCorner(c)

		for _,adj := range moves {
			ac := Translate(c,adj.dx,adj.dy)

			if ac.X == myHead.X && ac.Y == myHead.Y {
				continue
			}

			if ac.X < 0 || ac.X >= width || 
			   ac.Y < 0 || ac.Y >= height {
				vm[mx].sides++
				continue
			}

			adata := grid[ac.X][ac.Y]
			if IsHead(adata) {
				vm[mx].risky = true; 
			} else if IsBody(adata) {
				vm[mx].sides++
			}
		}

		maxFill := len(request.You.Body)
		filled := make(map[Coord]bool)
		var FillArea func (Coord) int
		FillArea = func (s Coord) int {
			_, alreadyFilled := filled[s]
			if alreadyFilled { return 0 }

			count := 1
			filled[s] = true

			west := Translate(s,-1,0)
			if west.X >= 0 && IsSpace(grid[west.X][west.Y]) {
				count += FillArea(west)
				if count > maxFill { return count }
			} 

			north := Translate(s,0,-1)
			if north.Y >= 0 && IsSpace(grid[north.X][north.Y]) {
				count += FillArea(north)
				if count > maxFill { return count }
			}

			east := Translate(s,+1,0)
			if east.X < width && IsSpace(grid[east.X][east.Y]) {
				count += FillArea(east)
				if count > maxFill { return count }
			}

			south := Translate(s,0,+1)
			if south.Y < height && IsSpace(grid[south.X][south.Y]) {
				count += FillArea(south)
			}

			return count
		}

		var SelfBounded func (Coord) bool
		SelfBounded = func (s Coord) bool {
			_, alreadyFilled := filled[s]
			if alreadyFilled { return true }

			filled[s] = true

			west := Translate(s,-1,0)
			if west.X >= 0 {
				c := grid[west.X][west.Y]
				if IsSpace(c) {
					if !SelfBounded(west) { return false }
				} else if !IsSelf(c) { return false }
			} 

			north := Translate(s,0,-1)
			if north.Y >= 0 {
				c := grid[north.X][north.Y]
				if IsSpace(c) {
					if !SelfBounded(north) { return false }
				} else if !IsSelf(c) { return false; }
			}

			east := Translate(s,+1,0)
			if east.X < width {
				c := grid[east.X][east.Y]
				if IsSpace(c) {
					if !SelfBounded(east) { return false }
				} else if !IsSelf(c) { return false }
			}

			south := Translate(s,0,+1)
			if south.Y < height {
				c := grid[south.X][south.Y]
				if IsSpace(c) {
					if !SelfBounded(south) { return false }
				} else if !IsSelf(c) { return false }
			}

			return true
		}

		if vm[mx].sides == 3 {
			numvm--
			fmt.Printf("[COLOR=%s, Reject %s: moving into trap]\n", color, move.label);
			continue;
		} else if vm[mx].sides > 0 {
			// determine if we would be entering a closed region whose number of tiles
			// is smaller than the current snake length
			area := FillArea(c)
			if area <= len(request.You.Body) {
				fmt.Printf("[COLOR=$s, Bounded region of size %d]\n", color, area)
				vm[mx].trap = true
				vm[mx].risky = true
				for k := range filled {
					delete(filled,k)
				}
				vm[mx].risky = true
				if (SelfBounded(c)) {
					vm[mx].region = -area
				} else {
					vm[mx].region = area
				}
			}
		}

		if IsFood(cdata) {
			fmt.Printf("[COLOR=%s, Food at %s]\n", color, move.label);
			vm[mx].dist = 0
			continue
		}

		// Compute distance to closest food
		vm[mx].dist = height + width
		for _,food := range fv {
			distToHere := ManDist(food.pos,myHead)
			distToNew := ManDist(food.pos,c)
			if distToNew < distToHere {
				vm[mx].dist = distToNew 
				break
			}
		}
		fmt.Printf("[COLOR=%s, Dist to closest food is %d]\n", color, vm[mx].dist);
	}

	lowHealth := request.You.Health < fv[len(fv)-1].dist
	criticalHealth := request.You.Health < 10

	// Choose among valid moves
	var chosenMove = "left"
	fmt.Printf("[COLOR=%s, Consider %d valid moves\n", color, numvm)
	switch numvm {
	case 0:
		chosenMove = "left"
		fmt.Printf("[COLOR=%s, No valid moves, suicide: %s]\n", color, chosenMove)
	case 1:
		chosenMove = vm[0].label
		fmt.Printf("[COLOR=%s, Only one valid move: %s]\n", color, chosenMove)
	case 2:
		// if both moves are traps, pick the one with the best chance of escape
		if vm[0].trap && vm[1].trap {
			if vm[0].region > 0 {
				if vm[1].region < vm[0].region { 
					chosenMove = vm[0].label
				} else {
					chosenMove = vm[1].label
				}
			} else if vm[1].region > 0 {
				chosenMove = vm[1].label
			} else if vm[0].region < vm[1].region {
				chosenMove = vm[0].label
			} else {
				chosenMove = vm[1].label
			}
			fmt.Printf("[COLOR=%s, Two moves, both traps, pick one with greater chance: %s]\n", color, chosenMove)
		} else if vm[0].trap {
			chosenMove = vm[1].label
		} else if vm[1].trap {
			chosenMove = vm[0].label
		} else {
			// if both moves are risky, pick the one with fewer sides
			if vm[0].risky && vm[1].risky {
				if vm[0].sides < vm[1].sides {
					chosenMove = vm[0].label
				} else {
					chosenMove = vm[1].label
				}
				fmt.Printf("[COLOR=%s, Two moves, both risky.  Choose one with fewer sides: %s]\n", color, chosenMove);
			} else {
				// if one is risky, pick the other
				if (vm[0].risky) {
					chosenMove = vm[1].label
					fmt.Printf("[COLOR=%s, Two moves, one risky.  Choose the other one: %s]\n", color, chosenMove);
				} else if vm[1].risky {
					chosenMove = vm[0].label
					fmt.Printf("[COLOR=%s, Two moves, one risky.  Choose the other one: %s]\n", color, chosenMove);
				} else {
					// FORCE the checks below to seek food (can reverse later if needed)
					criticalHealth = true

					// if one has food, choose that one
					// but only if sides < 2 or criticalHealth
					// note exception is when the cell is one of the corners
					if vm[0].dist == 0 && (criticalHealth || vm[0].sides < 2 || vm[0].corner) {
						chosenMove = vm[0].label
						fmt.Printf("[COLOR=%s, Two moves, one has food.  Choose it: %s]\n", color, chosenMove);
					} else if vm[1].dist == 0 && (criticalHealth || vm[1].sides < 2 || vm[1].corner) {
						chosenMove = vm[1].label
						fmt.Printf("[COLOR=%s, Two moves, one has food.  Choose it: %s]\n", color, chosenMove);
					} else {
						if (lowHealth) {
							// chooose the one with smaller dist
							if vm[0].dist < vm[1].dist {
								chosenMove = vm[0].label
								fmt.Printf("[COLOR=%s, Two moves, low health, one has lower dist.  Choose it: %s]\n", color, chosenMove);
							} else if vm[1].dist < vm[0].dist {
								chosenMove = vm[1].label
								fmt.Printf("[COLOR=%s, Two moves, low health, one has lower dist.  Choose it: %s]\n", color, chosenMove);
							} else {
								// choose the one with fewer sides
								if vm[0].sides < vm[1].sides {
									chosenMove = vm[0].label
									fmt.Printf("[COLOR=%s, Two moves, low health, one has fewer sides.  Choose it: %s]\n", color, chosenMove);
								} else {
									chosenMove = vm[1].label
									fmt.Printf("[COLOR=%s, Two moves, low health, one has fewer sides.  Choose it: %s]\n", color, chosenMove);
								}
							}
						} else {
							// if one has 2 sides, choose the other one
							if vm[1].sides == 2 {
								chosenMove = vm[0].label
								fmt.Printf("[COLOR=%s, Two moves, one has fewer sides.  Choose it: %s]\n", color, chosenMove);
							} else if vm[0].sides == 2 {
								chosenMove = vm[1].label
								fmt.Printf("[COLOR=%s, Two moves, one has fewer sides.  Choose it: %s]\n", color, chosenMove);
							} else {
								// choose the one with smaller dist
								if vm[0].dist < vm[1].dist {
									chosenMove = vm[0].label
									fmt.Printf("[COLOR=%s, Two moves, one has lower dist.  Choose it: %s]\n", color, chosenMove);
								} else {
									chosenMove = vm[1].label
									fmt.Printf("[COLOR=%s, Two moves, one has lower dist.  Choose it: %s]\n", color, chosenMove);
								}
							}
						}
					}
				}
			}
		}
	default:
		// if all moves are traps, choose the one with best chance of escape
		// if all moves risky, choose the one with the fewest sides
		if vm[0].trap && vm[1].trap && vm[2].trap {
			if vm[0].region < 0 && vm[1].region < 0 && vm[2].region < 0 {
				least := 0
				if (vm[1].region < vm[least].region) { least = 1 }
				if (vm[2].region < vm[least].region) { least = 2 }
				chosenMove = vm[least].label
			} else {
				most := 0
				if (vm[1].region > vm[most].region) { most = 1 }
				if (vm[2].region > vm[most].region) { most = 2 }
				chosenMove = vm[most].label
			}
		} else if vm[0].trap && vm[1].trap {
			chosenMove = vm[2].label
		} else if vm[0].trap && vm[2].trap {
			chosenMove = vm[1].label
		} else if vm[1].trap && vm[2].trap {
			chosenMove = vm[0].label
		} else {
			if vm[0].risky && vm[1].risky && vm[2].risky {
				least := 0
				if (vm[0].trap) { least = 1 }
				if (vm[1].sides < vm[least].sides && !vm[1].trap) { least = 1 }
				if (vm[2].sides < vm[least].sides && !vm[2].trap) { least = 2 }
				chosenMove = vm[least].label
				fmt.Printf("[COLOR=%s, Three moves, all risky.  Choose one with fewest sides: %s]\n", color, chosenMove);
			} else {
				// if two are risky, choose the other one unless its a trap
				if vm[0].risky && vm[1].risky {
					if vm[2].trap { 
						chosenMove = vm[0].label 
					} else { chosenMove = vm[2].label }
					fmt.Printf("[COLOR=%s, Three moves, two risky.  Choose the non-risky one: %s]\n", color, chosenMove);
				} else if vm[0].risky && vm[2].risky {
					if vm[1].trap { 
						chosenMove = vm[0].label 
					} else { chosenMove = vm[1].label }
					fmt.Printf("[COLOR=%s, Three moves, two risky.  Choose the non-risky one: %s]\n", color, chosenMove);
				} else if vm[1].risky && vm[2].risky {
					if vm[0].trap { 
						chosenMove = vm[1].label 
					} else { chosenMove = vm[0].label }
					fmt.Printf("[COLOR=%s, Three moves, two risky.  Choose the non-risky one: %s]\n", color, chosenMove);
				} else {
					// FORCE the checks below to seek food (can reverse later if needed)
					criticalHealth = true

					// if one has food, pick that one
					if vm[0].dist == 0 && !vm[0].risky && (criticalHealth || vm[0].sides < 2 || vm[0].corner) {
						chosenMove = vm[0].label
						fmt.Printf("[COLOR=%s, Three moves, one has food.  Choose it: %s]\n", color, chosenMove);
					} else if vm[1].dist == 0 && !vm[1].risky && (criticalHealth || vm[1].sides < 2 || vm[1].corner) {
						chosenMove = vm[1].label
						fmt.Printf("[COLOR=%s, Three moves, one has food.  Choose it: %s]\n", color, chosenMove);
					} else if vm[2].dist == 0 && !vm[2].risky && (criticalHealth || vm[2].sides < 2 || vm[2].corner) {
						chosenMove = vm[2].label
						fmt.Printf("[COLOR=%s, Three moves, one has food.  Choose it: %s]\n", color, chosenMove);
					} else {				
						if (lowHealth) {
							// if dist is all the same, then choose the non-risky one with fewest sides
							if vm[0].dist == vm[1].dist && vm[1].dist == vm[2].dist {
								least := 0
								if vm[0].risky { least = 1 }
								if vm[1].sides < vm[least].sides && !vm[1].risky { least = 1 }
								if vm[2].sides < vm[least].sides && !vm[2].risky { least = 2 }
								chosenMove = vm[least].label
								fmt.Printf("[COLOR=%s, Three moves, low health, pick non-risky one with least sides: %s]\n", color, chosenMove);
							} else {
								// choose the non-risky one with the least dist
								least := 0
								if vm[0].risky { least = 1 }
								if vm[1].dist < vm[least].dist && !vm[1].risky { least = 1 }
								if vm[2].dist < vm[least].dist && !vm[2].risky { least = 2 }
								chosenMove = vm[least].label
								fmt.Printf("[COLOR=%s, Three moves, low health, pick non-risky one with least dist: %s]\n", color, chosenMove);
							}
						} else {
							// if all have equal sides, or none have more than 1, then choose the non-risky one with smallest dist
							if vm[0].sides == vm[1].sides && vm[1].sides == vm[2].sides ||
							vm[0].sides < 2 && vm[1].sides < 2 && vm[2].sides < 2 {
								least := 0
								if vm[0].risky { least = 1 }
								if (vm[1].dist < vm[least].dist && !vm[1].risky) { least = 1 }
								if (vm[2].dist < vm[least].dist && !vm[2].risky) { least = 2 }
								chosenMove = vm[least].label
								fmt.Printf("[COLOR=%s, Three moves, pick non-risky one with least dist: %s]\n", color, chosenMove);
							} else {
								// choose the non-risky one with least number of sides
								least := 0
								if vm[0].risky {
									least = 1
								}
								if (vm[1].sides < vm[least].sides && !vm[1].risky) { least = 1 }
								if (vm[2].sides < vm[least].sides && !vm[2].risky) { least = 2 }
								chosenMove = vm[least].label
								fmt.Printf("[COLOR=%s, Three moves, pick non-risky one with least sides: %s]\n", color, chosenMove);
							}
						}
					}
				}
			}
		}
	} 

	response := MoveResponse { chosenMove,
							   "", // shout
							 }

	elapsed := time.Since(start)
	fmt.Printf("MOVE: COLOR=%s, Direction=%s, Elapsed=%dms\n", color, response.Move, elapsed.Milliseconds())
	
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
