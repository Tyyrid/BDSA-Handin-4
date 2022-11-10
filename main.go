package main

import (
	ping "DISYS-handin4/grpc"
	"bufio"
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"time"
)

var sureCounter int
var higherIdCounter int
var lowestTimeCounter int

func main() {

	arg1, _ := strconv.ParseInt(os.Args[1], 10, 32)
	ownPort := int32(arg1) + 5000

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := &peer{
		id:               ownPort,
		amountOfPings:    make(map[int32]int32),
		clients:          make(map[int32]ping.PingClient),
		ctx:              ctx,
		lamportTimestamp: 1,
	}

	// Create listener tcp on port ownPort
	list, err := net.Listen("tcp", fmt.Sprintf(":%v", ownPort))
	if err != nil {
		log.Fatalf("Failed to listen on port: %v", err)
	}
	grpcServer := grpc.NewServer()
	ping.RegisterPingServer(grpcServer, p)

	go func() {
		if err := grpcServer.Serve(list); err != nil {
			log.Fatalf("failed to server %v", err)
		}
	}()

	for i := 0; i < 3; i++ {
		port := int32(5000) + int32(i)

		if port == ownPort {
			continue
		}

		var conn *grpc.ClientConn
		fmt.Printf("Trying to dial: %v\n", port)
		conn, err := grpc.Dial(fmt.Sprintf(":%v", port), grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			log.Fatalf("Could not connect: %s", err)
		}
		defer conn.Close()
		c := ping.NewPingClient(conn)
		p.clients[port] = c
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		p.input = scanner.Text()
		if p.input == "can" {
			p.wantsCar = true
			fmt.Println("Der er blevet spurgt om bilen")
			//kun sende hvis en ønsker bilen
			p.askForCar()
		}

	}
}

type peer struct {
	ping.UnimplementedPingServer
	id            int32
	amountOfPings map[int32]int32
	clients       map[int32]ping.PingClient
	ctx           context.Context
}

func (p *peer) Ping(ctx context.Context, req *ping.Request) (*ping.Reply, error) {
	id := req.Id
	p.amountOfPings[id] += 1

	rep := &ping.Reply{Amount: p.amountOfPings[id]}
	return rep, nil
}

func (p *peer) criticalSection_GetTheCar(id int32) {
	log.Printf("******** Peer %d entered critical section at!!!!!! ********* ", id)
	time.Sleep(2)
	p.wantsCar = false
	sureCounter = 0
	higherIdCounter = 0
	lowestTimeCounter = 0
}

func (p *peer) askForCar() {
	var highestId int32
	var lowestTime int32
	var lowestTimeId int32
	//foregår hos en selv
	userInput := &ping.UserInput{
		ProcessId:        p.id,
		LamportTimeStamp: int32(p.lamportTimestamp) + 1,
		Input:            p.input,
	}

	for id, client := range p.clients {
		request, err := client.AnswerRequest(p.ctx, userInput)
		if err != nil {
			fmt.Printf("something went wrong %v", err.Error())
		}
		fmt.Printf("Got request from id %v: %v\n", id, request.RequestMsg)
		if request.RequestMsg == "sure" {
			sureCounter++
		}
		if request.RequestMsg == "No, I want it, because i have a higher ID" {
			//Find max id
			if highestId < request.ProcessId {
				highestId = request.ProcessId
			}
			higherIdCounter++
			fmt.Println("highest id %d", highestId)
			fmt.Println("highestidcounter %d", higherIdCounter)
		}

		//TODO: kig på det her
		if request.RequestMsg == "No, I want it, because i asked first" {

			if lowestTime > request.LamportTimeStamp {
				lowestTime = request.LamportTimeStamp
				lowestTimeId = request.ProcessId
			}
			if lowestTime == request.LamportTimeStamp {
				if request.ProcessId > lowestTimeId {
					lowestTimeId = request.ProcessId
				}
			}

			lowestTimeCounter++
			fmt.Printf("lowestTime %d", lowestTime)
		}
	}

	if sureCounter == 2 {
		p.criticalSection_GetTheCar(p.id)
	}
	if higherIdCounter == 2 {
		p.criticalSection_GetTheCar(highestId)

	}
	if lowestTimeCounter == 2 {
		p.criticalSection_GetTheCar(lowestTimeId)

	} else {
		sureCounter = 0
		lowestTimeCounter = 0
		higherIdCounter = 0
	}

}

func randomBool() bool {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(2) == 1
}
