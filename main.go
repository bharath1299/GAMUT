package main

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// global variables used to control the main execution flow, store local node information, and maintain a reference to the current node.
var running, created bool
var localPort string
var node *Node
var add NodeAddress

// variables store the command line arguments for configuring the DHT.
var address *string
var addressPort *int
var joinAddress *string
var joinPort *int
var timeStablize *int
var timeFixFingers *int
var timeCheckPredecessor *int
var successorAmount *int
var identifier *string

// constant representing the size of the finger table
var FingerTableSize = 10

// mutex (m) and a map (connections) are used to manage concurrent access to the connections.
var m sync.Mutex
var connections = make(map[string]*rpc.Client)

func main() {

	address = flag.String("a", "", "Address")
	addressPort = flag.Int("p", -1, "Port")
	joinAddress = flag.String("ja", "", "Join address")
	joinPort = flag.Int("jp", -1, "Join port")
	timeStablize = flag.Int("ts", -1, "Time before stabilize call")
	timeFixFingers = flag.Int("tff", -1, "Time before fix fingers call")
	timeCheckPredecessor = flag.Int("tcp", -1, "Time before check predecessor is called")
	successorAmount = flag.Int("r", -1, "The amount of immediate successor stored")
	identifier = flag.String("i", "", "The string identifier of a node")

	flag.Parse()
	*address = strings.TrimSpace(*address)
	*joinAddress = strings.TrimSpace(*joinAddress)
	*identifier = strings.TrimSpace(*identifier)
	localPort = ":" + strconv.Itoa(*addressPort)

	if (len(*joinAddress) == 0 && *joinPort >= 0) || (len(*joinAddress) > 0 && *joinPort < 0) {
		fmt.Printf("You have to provide both -ja and -jp flags")
		return
	}
	if (*addressPort < 0 || *timeStablize < 1 || *timeCheckPredecessor < 1 || *timeFixFingers < 1) ||
		(*timeStablize > 60000 || *timeCheckPredecessor > 60000 || *timeFixFingers > 60000) {
		fmt.Println("Invalid arguments")
		return
	}

	add = NodeAddress(*address + localPort)
	node = &Node{
		Address:     add,
		Successors:  []NodeAddress{},
		Predecessor: "",
		FingerTable: []NodeAddress{},
		Bucket:      make(map[Key]string),
	}

	server(*address, localPort)

	if len(*joinAddress) > 0 && *joinPort > 0 {
		//Joining an existing ring
		add := NodeAddress(*joinAddress + ":" + strconv.Itoa(*joinPort))
		join(add)
	} else {
		//Creating a new ring
		args := []string{*address + localPort}
		create(args)
	}

	go loopCP(time.Duration(*timeCheckPredecessor))
	go loopStab(time.Duration(*timeStablize))
	go loopFF(time.Duration(*timeFixFingers))

	res := bufio.NewReader(os.Stdin)

	var str string
	running = true
	created = false

	cmd := make(map[string]func([]string))
	cmd["LookUp"] = LookUp
	cmd["StoreFile"] = StoreFile
	cmd["PrintState"] = PrintState
	cmd["quit"] = quit
	cmd["dump"] = dump

	for running {
		fmt.Println("Enter Command: i.e. StoreFile <file name>")
		fmt.Print("> ")
		str, _ = res.ReadString('\n')
		str = strings.TrimSpace(str)
		args := strings.Split(str, " ")

		input, matched := cmd[args[0]]
		if matched {
			input(args) //holds a reference to a function and initiating it.
		} else {
			fmt.Println("Enter Command Properly.")
		}
	}
}

// function to create a new Chord ring
func create(args []string) {
	if created {
		fmt.Println("Node already created")
		return
	}
	node.create()
}

// A function to store a file in the Chord DHT. The file is encrypted using AES before storage.
func StoreFile(args []string) {

	filename := args[1]

	EncryptFile([]byte("a very very very very secret key"), filename, filename)

	fileData, err := os.ReadFile(filename)
	if err != nil {
		fmt.Println("file open error: " + err.Error())
	}

	content := string(fileData)

	add := findFile(args)

	//if the address maps to itself then there is no need to make a call. Encryption is done
	if strings.Compare(add, string(node.Address)) == 0 {
		return
	}

	reply := Reply{}
	arguments := Args{Command: content, Address: string(node.Address), Filename: filename, Offset: 0}

	ok := call(string(add), "Node.Store", &arguments, &reply)
	if !ok {
		fmt.Println("cannot reach the node")
		return
	}

}

// A function to encrypt the contents of a file using AES encryption.
func EncryptFile(key []byte, filename string, out string) {

	fileData, err := os.Open(filename)
	if err != nil {
		log.Printf("Error openening file.")
	}

	content, err := io.ReadAll(fileData)
	if err != nil {
		log.Printf("Error reading file.")
	}

	fileData.Close()

	enc, err := EncryptMessage(key, string(content))
	if err != nil {
		log.Printf("Error encrypting message.")
	}

	encodeData := base64.StdEncoding.EncodeToString(enc)

	outFile, err := os.Create(out)
	if err != nil {
		log.Printf("Error creating file.")
	}

	outFile.Write([]byte(encodeData))

	outFile.Close()
}

// A function to encrypt a message using AES encryption
func EncryptMessage(key []byte, message string) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	encryptedData := aesGCM.Seal(nonce, nonce, []byte(message), nil)

	return encryptedData, nil
}

// A function to find the node responsible for storing a file in the Chord DHT.
func findFile(args []string) string {
	filename := args[1]

	reply := Reply{}
	arguments := Args{Command: "", Address: filename, Offset: 0}

	add := node.Address
	flag := false

	for !flag {
		ok := call(string(add), "Node.FindSuccessor", &arguments, &reply)
		if !ok {
			fmt.Println("Failed to find successor") //change the print string
			return ""
		}
		switch found := reply.Found; found {

		//if the file maps between self and successor then reply.Reply = node.Successor[0]
		case true:
			flag = true
		//if the file maps somewhere else then we have to forward the request to a better node
		case false:
			add = NodeAddress(reply.Forward)
		}
	}
	return reply.Reply
}

// A function to look up the node responsible for storing a file in the Chord DHT.
func LookUp(args []string) {
	add := findFile(args)
	fmt.Println(hashAddress(NodeAddress(add)), add)

	SendRequest(add, args[1])

}

// A function to periodically check the predecessor of the current node.
func loopCP(t time.Duration) {
	for {
		cp([]string{})
		time.Sleep(t * time.Millisecond)
	}

}

// A function to periodically fix the finger table of the current node.
func loopFF(t time.Duration) {
	for {
		fix_fingers()
		time.Sleep(t * time.Millisecond)
	}

}

// A function to periodically stabilize the Chord ring.
func loopStab(t time.Duration) {

	for {
		stabilize([]string{})
		time.Sleep(t * time.Millisecond)

	}

}

// A function to gracefully quit the Chord DHT, closing connections and cleaning up resources
func quit(args []string) {
	running = false
	m.Lock()
	defer m.Unlock()
	fmt.Println(len(connections))
	for add, conn := range connections {
		err := conn.Close()
		if err != nil {
			fmt.Println("error closing :", add, err)
		}
	}
	fmt.Print("Quitting!\n")
}

// A function to handle predecessor checks.
func cp(args []string) {
	arguments := Args{Command: "CP", Address: string(node.Address), Offset: 0}
	reply := Reply{}

	if string(node.Predecessor) == "" {
		return
	}

	ok := call(string(node.Predecessor), "Node.HandlePing", &arguments, &reply)
	if !ok {
		node.mu.Lock()
		fmt.Println("Can not connect to predecessor")
		node.Predecessor = NodeAddress("")
		node.mu.Unlock()
		return
	}
}

// A function to fix the finger table of the current node.
func fix_fingers() {
	if len(node.FingerTable) == 0 {
		node.mu.Lock()
		node.FingerTable = []NodeAddress{node.Successors[0]}
		node.mu.Unlock()
		return
	}

	node.mu.Lock()
	node.FingerTable = []NodeAddress{}
	node.mu.Unlock()
	for next := 1; next <= FingerTableSize; next++ {
		offset := int64(math.Pow(2, float64(next)-1))
		add := node.Address
		flag := false
		for !flag {
			reply := Reply{}
			args := Args{Command: "", Address: string(node.Address), Offset: offset}
			ok := call(string(add), "Node.FindSuccessor", &args, &reply)
			if !ok {
				fmt.Println("Failed to fix fingers : ")
				return
			}

			switch found := reply.Found; found {
			case true:
				node.mu.Lock()

				node.FingerTable = append(node.FingerTable, NodeAddress(reply.Reply))
				flag = true
				node.mu.Unlock()
			case false:
				if strings.Compare(reply.Forward, string(node.Address)) == 0 {
					node.mu.Lock()
					flag = true
					node.FingerTable = append(node.FingerTable, NodeAddress(reply.Forward))
					node.mu.Unlock()
					break
				}
				add = NodeAddress(reply.Forward)
			}
		}
	}
}

// A function to stabilize the Chord ring
func stabilize(args []string) {
	arguments := Args{Command: "", Address: string(node.Address), Offset: 0}
	reply := Reply{}

	ok := call(string(node.Successors[0]), "Node.Get_predecessor", &arguments, &reply)
	if !ok {
		fmt.Println("Could not connect to predecessor")
		dump([]string{})
		node.mu.Lock()
		node.Successors = node.Successors[1:]
		if len(node.Successors) == 0 {
			node.Successors = []NodeAddress{node.Address}
		}
		node.mu.Unlock()
		return
	}
	node.mu.Lock()
	addH := hashAddress(node.Address)                 // Current node
	addressH := hashAddress(NodeAddress(reply.Reply)) // Predecessor
	succH := hashAddress(node.Successors[0])          // Successor

	if Inbetween(addH, addressH, succH, true) && reply.Reply != "" {
		node.Successors = []NodeAddress{NodeAddress(reply.Reply)}
	}

	node.mu.Unlock()
	arguments = Args{Command: "", Address: string(node.Address), Offset: 0}
	reply = Reply{}
	ok = call(string(node.Successors[0]), "Node.Get_successors", &arguments, &reply)
	if !ok {
		fmt.Println("Call failed to successor while stabilizing")
	}
	node.mu.Lock()

	node.Successors = []NodeAddress{node.Successors[0]}
	node.Successors = append(node.Successors, reply.Successors...)
	if len(node.Successors) > *successorAmount {
		node.Successors = node.Successors[:*successorAmount]
	}
	node.mu.Unlock()

	arguments = Args{Command: "Stabilize", Address: string(node.Address), Offset: 0}
	reply = Reply{}
	notify([]string{})
}

// A function to notify the successor of the current node.
func notify(args []string) {
	arguments := Args{Command: "Notify", Address: string(node.Address), Offset: 0}
	reply := Reply{}

	ok := call(string(node.Successors[0]), "Node.Notify", &arguments, &reply)
	if !ok {
		fmt.Println("Call failed in notify")
	}
}

// A function to start the RPC server for the current node.
func server(address string, port string) {
	rpc.Register(node)
	rpc.HandleHTTP()
	l, err := net.Listen("tcp", port)
	if err != nil {
		panic("Error starting RPC")
	}
	go http.Serve(l, nil)
	fmt.Println("Created node at address: " + address + localPort)
}

// A function to join an existing Chord ring.
func join(address NodeAddress) {
	reply := Reply{}
	args := Args{Command: "", Address: string(node.Address), Offset: 0}

	add := address
	flag := false

	for !flag {
		call(string(add), "Node.FindSuccessor", &args, &reply)

		fmt.Println(reply.Successors)

		switch found := reply.Found; found {
		case true:
			node.join(NodeAddress(reply.Reply))
			//fmt.Println("True")
			flag = true
		case false:
			add = NodeAddress(reply.Forward)
		}
	}
}

// A function to make RPC calls to other nodes in the Chord DHT.
func call(address string, method string, args interface{}, reply interface{}) bool {
	m.Lock()
	defer m.Unlock()

	c, ok := connections[address]
	if ok { // if already connection to address
		err := c.Call(method, args, reply)
		if err == nil {
			return true
		}

		fmt.Println("CALL ERROR: ", err)
		delete(connections, address)
		return false
	}

	c, err := rpc.DialHTTP("tcp", address)
	if err != nil {
		fmt.Println("error :", err)
		return false
	}
	connections[address] = c

	err = c.Call(method, args, reply)

	if err == nil {
		return true
	}

	fmt.Println("CALL ERROR: ", err)
	return false

}

// A function to print the state of the current node.
func dump(args []string) {
	fmt.Println("Address: " + node.Address)
	fmt.Println("ID: " + hashAddress(node.Address).String())
	fmt.Print("Finger table: ")
	fmt.Println(node.FingerTable)
	fmt.Println("Predecessor: " + node.Predecessor)
	fmt.Print("Successors: ")
	fmt.Println(node.Successors)
	fmt.Print("Bucket: ")
	fmt.Println(node.Bucket)
}

// A function to send a request for a file stored in the Chord DHT.
func SendRequest(address string, filename string) error {

	args := Args{Filename: filename}
	reply := Reply{}

	ok := call(address, "Node.GetFile", &args, &reply)
	if !ok {
		fmt.Println("Error requesting")
		return nil
	}

	text, err := DecryptMessage([]byte("a very very very very secret key"), reply.Content)
	if err != nil {
		fmt.Println("Error decrypting ", err)
		return nil

	}

	fmt.Println("Encoded content:", reply.Content)
	fmt.Println("Decrypted Text: ", text)
	return nil

}

// A function to decrypt a message using AES encryption.
func DecryptMessage(key []byte, message string) (string, error) {
	cipherText, err := base64.StdEncoding.DecodeString(message)
	if err != nil {
		return "", fmt.Errorf("could not base64 decode: %v", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher block: %v", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %v", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(cipherText) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, cipherData := cipherText[:nonceSize], cipherText[nonceSize:]
	decryptedData, err := aesGCM.Open(nil, nonce, cipherData, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt data: %v", err)
	}

	return string(decryptedData), nil
}

// A function to print the state of the Chord DHT.
func PrintState(args []string) {

	fmt.Println("OWN INFORMATION: ")
	fmt.Println(node.Address, hashAddress(node.Address))

	fmt.Println("PREDECESSOR: ")
	fmt.Println(node.Predecessor, hashAddress(node.Predecessor))

	fmt.Println("SUCCESSORS:")
	for _, s := range node.Successors {
		fmt.Println(s, hashAddress(s))
	}

	fmt.Println("FINGER TABLE:")
	for _, f := range node.FingerTable {
		fmt.Println(f, hashAddress(f))
	}
}
