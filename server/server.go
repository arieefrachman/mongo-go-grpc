package main

import (
	"context"
	"fmt"
	"github.com/arieefrachman/mongo-go/pb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"log"
	"net"
	"os"
	"os/signal"
)

var Collection *mongo.Collection

type BlogItem struct {
	ID primitive.ObjectID `bson:"_id,omitempty"`
	AuthorID string       `bson:"author_id"`
	Content string        `bson:"content"`
	Title string          `bson:"title"`
}
type server struct {}

func (*server) CreateBlog(ctx context.Context, request *pb.CreateBlogRequest) (*pb.CreateBlogResponse, error){
	r := request.GetBlog()
	
	data := BlogItem{
		AuthorID: r.AuthorId,
		Content:  r.Content,
		Title:    r.Title,
	}

	res, err := Collection.InsertOne(ctx, data)
	if err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("internal error: %v", err))
	}

	oid, ok := res.InsertedID.(primitive.ObjectID)
	if !ok {
		return nil, status.Errorf(codes.Internal, "cannot convert OID")
	}
	return &pb.CreateBlogResponse{
		Blog: &pb.Blog{
			Id: oid.Hex(),
		},
	}, nil
}

func (*server) ReadBlog(ctx context.Context, request *pb.ReadBlogRequest) (*pb.ReadBlogResponse, error){
	blogID := request.BlogId
	oid, err := primitive.ObjectIDFromHex(blogID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, fmt.Sprintf("cannot parse ID"))
	}

	d := &BlogItem{}
	filter := bson.M{
		"_id": oid,
	}
	res := Collection.FindOne(context.Background(), filter)
	if err := res.Decode(d); err != nil {
		return nil, status.Errorf(codes.NotFound, fmt.Sprintf("document not found"))
	}

	return &pb.ReadBlogResponse{
		Blog: &pb.Blog{
			Id:                   d.ID.Hex(),
			AuthorId:             d.AuthorID,
			Title:                d.Title,
			Content:              d.Content,
		},
	}, nil
}
func main()  {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	fmt.Println("Blog service started...")

	client, errMongo := mongo.NewClient(options.Client().ApplyURI("mongodb://localhost:27017"))

	if errMongo != nil {
		log.Fatalf("failed to create connection: %v", errMongo)
	}

	errConn := client.Connect(context.TODO())

	if errConn != nil {
		log.Fatalf("failed to connect: %v", errConn)
	}

	Collection = client.Database("mydb").Collection("blog")
	listener, errLis := net.Listen("tcp", "localhost:50051")

	if errLis != nil {
		log.Fatalf("failed to listen: %v\n", errLis)
	}

	opts := []grpc.ServerOption{}

	s := grpc.NewServer(opts...)
	pb.RegisterBlogServiceServer(s, &server{})
	reflection.Register(s)
	go func() {
		fmt.Println("server is starting....")
		if err := s.Serve(listener); err != nil {
			log.Fatalf("failed to serve: %v\n", err)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	<-ch
	fmt.Println("stopping the server")
	s.Stop()
	fmt.Println("closing the listener")
	listener.Close()
	fmt.Println("closing the mongodb")
	client.Disconnect(context.TODO())

}
