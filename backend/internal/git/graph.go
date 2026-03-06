package git

import "strings"

// GraphData holds computed layout for one commit.
type GraphData struct {
	Color       string
	Column      int
	Connections []Connection
}

// Connection represents a line from one commit node to another.
type Connection struct {
	ToColumn int
	ToRow    int
	Color    string
}

// lane tracks which commit hash an active lane is expecting next.
// nil entries represent free (reusable) slots.
type lane struct {
	expectHash string
	color      string
}

// branch colors for the visual graph
var graphColors = []string{
	"#F14E32", // orange (primary)
	"#14B8A6", // teal
	"#8B5CF6", // purple
	"#F59E0B", // amber
	"#EC4899", // pink
	"#06B6D4", // cyan
	"#84CC16", // lime
	"#EF4444", // red
}

// ComputeGraph assigns columns and connections to a list of commits.
// Commits must be in topological order (newest first), as produced by git log --topo-order.
//
// Uses a two-pass approach:
//   - Pass 1: Assign columns to every commit using a lane-based algorithm with slot reuse.
//   - Pass 2: Build connections using actual assigned columns (avoids stale-index bugs).
func ComputeGraph(commits []RawCommit) map[string]GraphData {
	if len(commits) == 0 {
		return nil
	}

	hashToRow := make(map[string]int, len(commits))
	for i, c := range commits {
		hashToRow[c.Hash] = i
	}

	type assignedInfo struct {
		column int
		color  string
	}
	assigned := make(map[string]assignedInfo, len(commits))

	// Lanes use nil slots for free positions (reusable).
	var lanes []*lane
	colorIdx := 0

	nextColor := func() string {
		c := graphColors[colorIdx%len(graphColors)]
		colorIdx++
		return c
	}

	findLane := func(hash string) int {
		for i, l := range lanes {
			if l != nil && l.expectHash == hash {
				return i
			}
		}
		return -1
	}

	findFree := func() int {
		for i, l := range lanes {
			if l == nil {
				return i
			}
		}
		return len(lanes)
	}

	setLane := func(idx int, l *lane) {
		for len(lanes) <= idx {
			lanes = append(lanes, nil)
		}
		lanes[idx] = l
	}

	// Pre-reserve lane 0 for the main/master branch so it stays on the primary column.
	// Feature branches fork to the right.
	var reserveHash string
	for _, c := range commits {
		for _, ref := range strings.Split(c.Refs, ", ") {
			ref = strings.TrimSpace(ref)
			ref = strings.TrimPrefix(ref, "HEAD -> ")
			if ref == "main" || ref == "master" {
				reserveHash = c.Hash
				break
			}
		}
		if reserveHash != "" {
			break
		}
	}
	// Fallback to HEAD if no main/master branch found
	if reserveHash == "" {
		for _, c := range commits {
			if strings.Contains(c.Refs, "HEAD") {
				reserveHash = c.Hash
				break
			}
		}
	}
	if reserveHash != "" {
		setLane(0, &lane{expectHash: reserveHash, color: nextColor()})
	}

	// Pass 1: Assign a column to each commit.
	for _, c := range commits {
		col := findLane(c.Hash)
		var color string

		if col == -1 {
			// New branch head — allocate a free lane slot.
			col = findFree()
			color = nextColor()
			setLane(col, &lane{expectHash: c.Hash, color: color})
		} else {
			color = lanes[col].color
		}

		// Close any other lanes that also expect this commit (converging branches).
		for i, l := range lanes {
			if i != col && l != nil && l.expectHash == c.Hash {
				lanes[i] = nil
			}
		}

		assigned[c.Hash] = assignedInfo{column: col, color: color}

		if len(c.Parents) == 0 {
			// Root commit — close lane.
			lanes[col] = nil
		} else {
			// First parent continues in the same lane.
			lanes[col] = &lane{expectHash: c.Parents[0], color: color}

			// Additional parents (merge): find existing lane or allocate new one.
			for _, ph := range c.Parents[1:] {
				if findLane(ph) != -1 {
					continue // already tracked by another lane
				}
				newCol := findFree()
				pColor := nextColor()
				setLane(newCol, &lane{expectHash: ph, color: pColor})
			}
		}
	}

	// Pass 2: Build connections using the actual assigned columns.
	result := make(map[string]GraphData, len(commits))
	for _, c := range commits {
		ai := assigned[c.Hash]
		var connections []Connection

		for pIdx, ph := range c.Parents {
			parentRow, ok := hashToRow[ph]
			if !ok {
				continue
			}
			pai, ok := assigned[ph]
			if !ok {
				continue
			}

			// First parent = same branch continuation → use child's color.
			// Additional parents (merge) → use the parent's color.
			connColor := ai.color
			if pIdx > 0 {
				connColor = pai.color
			}

			connections = append(connections, Connection{
				ToColumn: pai.column,
				ToRow:    parentRow,
				Color:    connColor,
			})
		}

		result[c.Hash] = GraphData{
			Color:       ai.color,
			Column:      ai.column,
			Connections: connections,
		}
	}

	return result
}
