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
	"sync"
	"time"
)

var mu sync.Mutex

func main() {
	arg1, _ := strconv.ParseInt(os.Args[1], 10, 32)
	ownPort := int32(arg1) + 8080
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := &peer{
		id:               ownPort,
		clients:          make(map[int32]ping.PingClient),
		ctx:              ctx,
		lamportTimestamp: randomInt(),  //this will stay the same through the entire session
		wantsCar:         randomBool(), //this will stay the same through the entire session
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
		port := int32(8080) + int32(i)

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
		if p.input == "May I have the car?" {
			p.wantsCar = true
			fmt.Printf("I want the car. My Lamport timestamp is: %d\n", p.lamportTimestamp)
			//Peer requests car
			p.askForCar()
		}

	}
}

type peer struct {
	ping.UnimplementedPingServer
	id int32
	//Stores the other peers that are connected
	clients          map[int32]ping.PingClient
	ctx              context.Context
	lamportTimestamp int
	input            string
	wantsCar         bool
	sureCounter      int
}

func (p *peer) AnswerRequest(ctx context.Context, userinput *ping.UserInput) (*ping.Request, error) {
	msg := userinput.Input

	if p.wantsCar == false {
		fmt.Printf("I Dont want the car. My Lamport time stamp is: %d\n", p.lamportTimestamp)
		msg = "sure"

	} else {

		// Checks LamportTimeStamp
		if int32(p.lamportTimestamp) == userinput.LamportTimeStamp {
			//If they have the same LamportTimeStamp, checks an processID. Which one is the highest?
			if p.id > userinput.ProcessId {
				msg = "No, I want it, because i have a higher ID"
				fmt.Printf("No, I want it, because i have a higher ID. I will ask the others if I can get the car. My Lamport time stamp is: %d\n", p.lamportTimestamp)
				p.askForCar()
			} else {
				msg = "sure"
				fmt.Printf("sure. My lamportStamp is: %d\n", p.lamportTimestamp)
			}

		} else if int32(p.lamportTimestamp) < userinput.LamportTimeStamp {
			//p.lamportTimestamp får bilen
			msg = "No, I want it, because i asked first"
			fmt.Printf("No, I want it, because i asked first. I will ask the others if I can get the car. My Lamport time stamp is: %d\n", p.lamportTimestamp)
			p.askForCar()

		} else if int32(p.lamportTimestamp) > userinput.LamportTimeStamp {
			//userinput.LamportTimeStamp får bilen
			msg = "sure"
			fmt.Printf("sure. My Lamport timestamp is: %d\n", p.lamportTimestamp)
		}

	}

	//The message to be returned
	req := &ping.Request{
		RequestMsg: msg,
	}

	return req, nil
}

func (p *peer) criticalSection_GetTheCar(id int32) {
	//safety - only one at the time
	mu.Lock()
	log.Printf("******** Peer %d entered critical section - I HAVE THE CAR!!!!!! ********* ", id)
	time.Sleep(2)
	p.wantsCar = false
	defer mu.Unlock()
}

// Peer asks for permission to get the car
func (p *peer) askForCar() {
	userInput := &ping.UserInput{
		ProcessId:        p.id,
		LamportTimeStamp: int32(p.lamportTimestamp),
		Input:            p.input,
	}

	//Asks all the peers that are connected
	for id, client := range p.clients {
		request, err := client.AnswerRequest(p.ctx, userInput)
		if err != nil {
			fmt.Printf("something went wrong %v", err.Error())
		}
		fmt.Printf("Got reply from id %v: %v\n", id, request.RequestMsg)

		if request.RequestMsg == "sure" {
			p.sureCounter++
		}
	}

	//Checks if the peer has received two 'sure' to get the car
	if p.sureCounter == 2 {
		p.criticalSection_GetTheCar(p.id)
		p.sureCounter = 0
	}

	//Sets the counter to zero for all peers
	p.sureCounter = 0

}

// Sets a random value to wantsCar for each peer
func randomBool() bool {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(2) == 1
}

// Generates a random start number for Lamport timestamp between 1 and 4
func randomInt() int {
	min := 1
	max := 4
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(max-min) + min
}
