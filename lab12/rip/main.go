package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

const Infinity = 16

type Topology struct {
	Routers []string    `json:"routers"`
	Links   [][2]string `json:"links"`
}

type Route struct {
	Destination string
	NextHop     string
	Metric      int
}

type Router struct {
	IP        string
	Neighbors map[string]bool
	Table     map[string]Route
}

func loadTopology(filename string) (*Topology, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var topology Topology
	err = json.Unmarshal(data, &topology)
	if err != nil {
		return nil, err
	}

	return &topology, nil
}

func createRouters(topology *Topology) map[string]*Router {
	routers := make(map[string]*Router)

	for _, ip := range topology.Routers {
		routers[ip] = &Router{
			IP:        ip,
			Neighbors: make(map[string]bool),
			Table:     make(map[string]Route),
		}
	}

	for _, link := range topology.Links {
		a := link[0]
		b := link[1]

		routers[a].Neighbors[b] = true
		routers[b].Neighbors[a] = true
	}

	for _, router := range routers {
		router.Table[router.IP] = Route{
			Destination: router.IP,
			NextHop:     router.IP,
			Metric:      0,
		}

		for neighbor := range router.Neighbors {
			router.Table[neighbor] = Route{
				Destination: neighbor,
				NextHop:     neighbor,
				Metric:      1,
			}
		}
	}

	return routers
}

func runRIP(routers map[string]*Router) {
	changed := true
	iteration := 1

	for changed {
		changed = false

		fmt.Printf("Iteration %d\n", iteration)

		updates := make(map[string][]Route)

		for _, router := range routers {
			for neighborIP := range router.Neighbors {
				neighbor := routers[neighborIP]

				for _, neighborRoute := range neighbor.Table {
					newMetric := min(neighborRoute.Metric+1, Infinity)

					updates[router.IP] = append(updates[router.IP], Route{
						Destination: neighborRoute.Destination,
						NextHop:     neighborIP,
						Metric:      newMetric,
					})
				}
			}
		}

		for routerIP, routes := range updates {
			router := routers[routerIP]

			for _, route := range routes {
				if route.Destination == router.IP {
					continue
				}

				current, exists := router.Table[route.Destination]

				if !exists || route.Metric < current.Metric {
					router.Table[route.Destination] = route
					changed = true

					fmt.Printf(
						"Router %s updated route to %s via %s, metric %d\n",
						router.IP,
						route.Destination,
						route.NextHop,
						route.Metric,
					)
				}
			}
		}

		fmt.Println()
		iteration++
	}
}

func printTables(routers map[string]*Router) {
	routerIPs := make([]string, 0, len(routers))
	for ip := range routers {
		routerIPs = append(routerIPs, ip)
	}
	sort.Strings(routerIPs)

	for _, ip := range routerIPs {
		router := routers[ip]

		fmt.Printf("Final state of router %s table:\n", router.IP)
		fmt.Printf("%-18s %-18s %-18s %-8s\n", "[Source IP]", "[Destination IP]", "[Next Hop]", "[Metric]")

		destinations := make([]string, 0, len(router.Table))
		for destination := range router.Table {
			if destination != router.IP {
				destinations = append(destinations, destination)
			}
		}
		sort.Strings(destinations)

		for _, destination := range destinations {
			route := router.Table[destination]

			fmt.Printf(
				"%-18s %-18s %-18s %-8d\n",
				router.IP,
				route.Destination,
				route.NextHop,
				route.Metric,
			)
		}

		fmt.Println()
	}
}

func main() {
	filename := "topology.json"

	if len(os.Args) > 1 {
		filename = os.Args[1]
	}

	topology, err := loadTopology(filename)
	if err != nil {
		fmt.Println("Failed to load topology:", err)
		return
	}

	routers := createRouters(topology)

	fmt.Println("Starting RIP simulation")
	fmt.Println()

	runRIP(routers)

	fmt.Println("RIP converged")
	fmt.Println()

	printTables(routers)
}
