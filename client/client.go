/*
Copyright 2018 Alex Hunt

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"flag"
	pb "github.com/alexhunt7/gofigure/proto"
	"golang.org/x/net/context"
	"log"
	"time"
)

var (
	tls                = flag.Bool("tls", false, "Connection uses TLS if true, else plain TCP")
	caFile             = flag.String("ca_file", "", "The file containning the CA root cert file")
	serverAddr         = flag.String("server_addr", "127.0.0.1:10000", "The server address in the format of host:port")
	serverHostOverride = flag.String("server_host_override", "x.test.youtube.com", "The server name use to verify the hostname returned by TLS handshake")
)

type Client struct {
	pb.GofigureClient
}

func (client Client) Exec(executable string, args ...string) {
	log.Printf("runExec")
	request := &pb.ExecRequest{
		Executable: executable,
		Args:       args,
	}
	log.Printf("Instantiated request")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	log.Printf("Created context")
	response, err := client.GofigureExec(ctx, request)
	log.Printf("ran client.GofigureExec")
	if err != nil {
		log.Printf("failed to exec")
		log.Fatal(err)
	}
	log.Printf("stdout: %s", response.Stdout)
	log.Printf("stderr: %s", response.Stderr)
}

func (client Client) Stat(path string) {
	log.Printf("runStat")
	request := &pb.FilePath{
		Path: path,
	}
	log.Printf("Instantiated request")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	log.Printf("Created context")
	response, err := client.GofigureStat(ctx, request)
	log.Printf("ran client.GofigureStat")
	//log.Printf(response.Msg)
	if err != nil {
		log.Printf("failed to stat file")
		log.Fatal(err)
	}
	log.Printf("owner: %s", response.Owner)
	log.Printf("group: %s", response.Group)
	log.Printf("mode: %s", response.Mode)
}

func (client Client) CreateFile(path string) {
	log.Printf("runCreateDir")
	request := &pb.FileRequest{
		Properties: &pb.FileProperties{
			Path:  path,
			Owner: "alex",
			Group: "alex",
			Mode:  "666",
		},
		Content: []byte("this is a test\n"),
	}
	log.Printf("Instantiated request")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	log.Printf("Created context")
	//response, err := client.GofigureDirectory(ctx, request)
	_, err := client.GofigureFile(ctx, request)
	log.Printf("ran client.GofigureFile")
	//log.Printf(response.Msg)
	if err != nil {
		log.Printf("failed to create file")
		log.Fatal(err)
	}
}

func (client Client) CreateDir(path string) {
	log.Printf("runCreateDir")
	request := &pb.FileRequest{
		Properties: &pb.FileProperties{
			Path:  path,
			Owner: "alex",
			Group: "alex",
			Mode:  "700",
		},
	}
	log.Printf("Instantiated request")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	log.Printf("Created context")
	//response, err := client.GofigureDirectory(ctx, request)
	_, err := client.GofigureDirectory(ctx, request)
	log.Printf("ran client.GofigureDirectory")
	//log.Printf(response.Msg)
	if err != nil {
		log.Printf("failed to create dir")
		log.Fatal(err)
	}
}
