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

	//generates a random start number for Lamport timestamp
	/*min := 1
	max := 4
	rand.Seed(time.Now().UnixNano())
	myLogicalTimestamp := rand.Intn(max-min) + min*/

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
	//gemmer på de andre peers som er connected
	clients          map[int32]ping.PingClient
	ctx              context.Context
	lamportTimestamp int
	input            string
	wantsCar         bool
}

func (p *peer) AnswerRequest(ctx context.Context, userinput *ping.UserInput) (*ping.Request, error) {
	//foregår hos de andre peers
	processId := userinput.ProcessId
	logicalTime := userinput.LamportTimeStamp
	msg := userinput.Input

	//Generate random bool
	//p.wantsCar = randomBool()

	p.wantsCar = true

	if p.wantsCar == false {
		fmt.Println("I Dont want the car")
		msg = "sure"

	} else {

		// Checks LamportTimeStamp
		if int32(p.lamportTimestamp) == userinput.LamportTimeStamp {

			fmt.Printf("p.id = %v og userinputid = %v", p.id, userinput.ProcessId)
			//If they have the same LamportTimeStamp, checks an processID. Which one is the highest?
			if p.id > userinput.ProcessId {
				msg = "No, I want it, because i have a higher ID"
				processId = p.id
				//p.criticalSection_GetTheCar(p.id)
			} else {
				msg = "sure"

			}

		} else if int32(p.lamportTimestamp) < userinput.LamportTimeStamp {
			//p.lamportTimestamp får bilen
			msg = "No, I want it, because i asked first"
			processId = p.id
			//p.criticalSection_GetTheCar(p.id)

		} else if int32(p.lamportTimestamp) > userinput.LamportTimeStamp {
			//userinput.LamportTimeStamp får bilen
			msg = "sure"
		}

	}

	log.Printf("ping peer id: %s", processId)
	log.Printf("ping msg: %s", userinput.Input)
	log.Printf("ping Timestamp: %d", p.lamportTimestamp)

	req := &ping.Request{
		ProcessId:        processId,
		LamportTimeStamp: logicalTime,
		RequestMsg:       msg,
	}

	return req, nil
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
