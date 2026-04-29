package main

import (
	"fmt"
	"time"

	"github.com/remora-go/framework-paladin/paladin"
)

const gridSize = 20

type Point struct{ X, Y int }
type Direction struct{ DX, DY int }

type SnakeGame struct {
	snake      []Point
	direction  Direction
	food       Point
	score      int
	gameOver   bool
	paused     bool
}

var up = Direction{-1, 0}
var down = Direction{1, 0}
var left = Direction{0, -1}
var right = Direction{0, 1}

func main() {
	trace := paladin.NewTrace("snake_game")
	ctx := trace.Start()
	defer trace.Flush()

	game := initGame(ctx)
	game.runLoop(ctx)
	game.endGame(ctx)
}

func initGame(ctx *paladin.Context) *SnakeGame {
	child := ctx.Child("initGame")
	defer child.End()

	child.Actor("system", "inicializa el juego Snake Game")
	child.Goal("Preparar estado inicial del juego con serpiente en posicion central")

	child.Event("game_initialization", "Snake Game iniciando", map[string]any{"gridSize": gridSize})

	game := &SnakeGame{
		snake:     []Point{{X: gridSize / 2, Y: gridSize / 2}},
		direction: right,
		food:      Point{X: 5, Y: 5},
		score:     0,
		gameOver:  false,
	}

	child.Var("initialSnakeLength", len(game.snake))
	child.Var("initialPosition", fmt.Sprintf("%v", game.snake[0]))
	child.Var("foodPosition", fmt.Sprintf("%v", game.food))

	child.Rule("initial_snake_length", "La serpiente inicia con longitud 1", nil)
	child.Check("initial_snake_length", "snake_length == 1", fmt.Sprintf("snake_length == %d", len(game.snake)), len(game.snake) == 1)

	child.Expect("game_loop", "El juego entra en el loop principal")
	child.Handoff("initGame", "gameLoop", "Inicializacion completa")

	return game
}

func (g *SnakeGame) runLoop(ctx *paladin.Context) {
	child := ctx.Child("runLoop")
	defer child.End()

	child.Actor("game_loop", "controla el flujo del juego")
	child.Goal("Mantener el juego corriendo hasta game over")

	child.Event("game_start", "Loop principal iniciado", map[string]any{"maxIterations": 100})

	tickRate := 250 * time.Millisecond
	for i := 0; i < 10 && !g.gameOver; i++ {
		iteration := child.Child(fmt.Sprintf("tick_%d", i))
		
		iteration.Var("iteration", i)
		iteration.Var("score", g.score)
		iteration.Var("snakeLength", len(g.snake))

		g.update(iteration)

		if i < 9 {
			time.Sleep(tickRate)
		}

		iteration.Expect("next_tick", fmt.Sprintf("tick_%d", i+1))
		iteration.End()
	}
}

func (g *SnakeGame) update(ctx *paladin.Context) {
	child := ctx.Child("update")
	defer child.End()

	child.Event("frame_update", "Actualizando estado del juego", nil)

	// Calcular nueva posicion de la cabeza
	head := g.snake[0]
	newHead := Point{
		X: head.X + g.direction.DX,
		Y: head.Y + g.direction.DY,
	}

	child.Var("currentHead", fmt.Sprintf("%v", head))
	child.Var("newHead", fmt.Sprintf("%v", newHead))
	child.Var("direction", fmt.Sprintf("%v", g.direction))

	// Mover serpiente
	g.snake = append([]Point{newHead}, g.snake...)
	child.Event("snake_moved", "Serpiente se movio a nueva posicion", map[string]any{"newHead": fmt.Sprintf("%v", newHead)})

	// Check collision with walls
	wallCollision := newHead.X < 0 || newHead.X >= gridSize || newHead.Y < 0 || newHead.Y >= gridSize
	child.Check("wall_boundary", "head dentro de grid", fmt.Sprintf("X=%d, Y=%d", newHead.X, newHead.Y), !wallCollision)

	if wallCollision {
		child.Violation("wall_hit", "serpiente dentro del grid", fmt.Sprintf("serpiente en X=%d, Y=%d", newHead.X, newHead.Y))
		g.gameOver = true
		child.Decision("game_over_wall", "Colision con pared detecteda")
		return
	}

	// Check self collision
	selfCollision := false
	for _, segment := range g.snake[1:] {
		if segment.X == newHead.X && segment.Y == newHead.Y {
			selfCollision = true
			break
		}
	}

	child.Check("self_collision", "serpiente no colisiona consigo misma", fmt.Sprintf("newHead=%v", newHead), !selfCollision)

	if selfCollision {
		child.Violation("self_hit", "serpiente no choca consigo misma", fmt.Sprintf("serpiente choca en %v", newHead))
		g.gameOver = true
		child.Decision("game_over_self", "Colision consigo misma detectada")
		return
	}

	// Check food collision
	if newHead.X == g.food.X && newHead.Y == g.food.Y {
		child.Decision("eat_food", "Nueva posicion coincide con comida")
		g.score++
		child.Var("score", g.score)
		g.spawnFood(child)
	} else {
		g.snake = g.snake[:len(g.snake)-1]
		child.Event("snake_grew", "Serpiente no crecio, removiendo cola", nil)
	}

	child.Expect("next_frame", "Siguiente actualizacion")
}

func (g *SnakeGame) spawnFood(ctx *paladin.Context) {
	child := ctx.Child("spawnFood")
	defer child.End()

	child.Event("food_eaten", "Comida consumida, generando nueva", nil)

	// Simple food spawn - just move to random position
	g.food = Point{X: (g.food.X + 3) % gridSize, Y: (g.food.Y + 2) % gridSize}

	child.Var("newFoodPosition", fmt.Sprintf("%v", g.food))
	child.Rule("food_position", "Comida siempre dentro del grid", nil)
	child.Check("food_in_bounds", "comida dentro del grid", fmt.Sprintf("X=%d, Y=%d", g.food.X, g.food.Y), g.food.X >= 0 && g.food.X < gridSize && g.food.Y >= 0 && g.food.Y < gridSize)

	child.Handoff("spawnFood", "update", "Comida reubicada")
}

func (g *SnakeGame) endGame(ctx *paladin.Context) {
	child := ctx.Child("endGame")
	defer child.End()

	child.Actor("system", "finaliza el juego")
	child.Event("game_over", "Juego terminado", map[string]any{"finalScore": g.score, "finalSnakeLength": len(g.snake)})

	child.Var("finalScore", g.score)
	child.Var("totalTicks", 10)

	child.Rule("end_score", "Puntuacion final registrada", nil)
	child.Check("final_score_recorded", "score >= 0", fmt.Sprintf("score == %d", g.score), g.score >= 0)

	child.Handoff("endGame", "main", "Juego finalizado")

	fmt.Printf("\n=== GAME OVER ===\n")
	fmt.Printf("Final Score: %d\n", g.score)
	fmt.Printf("Final Snake Length: %d\n", len(g.snake))
	fmt.Printf("====================\n\n")
}