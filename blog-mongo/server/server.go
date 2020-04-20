package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/silviog1990/grpc-golang-course/blog-mongo/blogpb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

type server struct{}

type blogItem struct {
	ID      primitive.ObjectID `bson:"_id,omitempty"`
	Author  string             `bson:"author"`
	Title   string             `bson:"title"`
	Content string             `bson:"content"`
}

func (data *blogItem) transformToBlog() *blogpb.Blog {
	if !data.ID.IsZero() {
		return &blogpb.Blog{
			Id:      data.ID.Hex(),
			Author:  data.Author,
			Title:   data.Title,
			Content: data.Title,
		}
	}
	return &blogpb.Blog{
		Author:  data.Author,
		Title:   data.Title,
		Content: data.Title,
	}
}

func (data *blogItem) transformFromBlog(blog *blogpb.Blog) *blogItem {
	return &blogItem{
		Author:  blog.Author,
		Title:   blog.Title,
		Content: blog.Content,
	}
}

var db *mongo.Database
var blogCollection *mongo.Collection

func (*server) CreateBlog(ctx context.Context, req *blogpb.CreateBlogRequest) (*blogpb.CreateBlogResponse, error) {
	fmt.Println("Create blog invoked")
	blog := req.GetBlog()
	if blog == nil {
		return nil, status.Error(
			codes.InvalidArgument, "blog nil",
		)
	}

	blogItem := &blogItem{}
	blogItem = blogItem.transformFromBlog(blog)

	res, err := blogCollection.InsertOne(ctx, blogItem)
	if err != nil {
		return nil, err
	}
	blog.Id = res.InsertedID.(primitive.ObjectID).Hex()
	return &blogpb.CreateBlogResponse{Blog: blog}, nil
}

func (*server) ReadBlog(ctx context.Context, req *blogpb.ReadBlogRequest) (*blogpb.ReadBlogResponse, error) {
	fmt.Println("ReadBlog invoked")
	blogID := req.Id
	primitiveBlogID, err := primitive.ObjectIDFromHex(blogID)
	if err != nil {
		return nil, status.Errorf(
			codes.InvalidArgument,
			fmt.Sprintf("Cannot parse passed id: %v", blogID),
		)
	}

	data := &blogItem{}
	filter := bson.M{"_id": primitiveBlogID}

	blogFound := blogCollection.FindOne(ctx, filter)
	if err := blogFound.Decode(data); err != nil {
		return nil, status.Error(
			codes.NotFound,
			fmt.Sprintf("Blog not found with id: %v", blogID),
		)
	}
	blogRes := data.transformToBlog()
	return &blogpb.ReadBlogResponse{
		Blog: blogRes,
	}, nil
}

func main() {
	// if we crash the go code, we get the file name and line number
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// connection with mongodb
	fmt.Println("Connecting to database...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	clientDB, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://mongoadmin:passwordMongo2020@localhost:27017"))
	db = clientDB.Database("grpc_course")
	blogCollection = db.Collection("blogs")
	fmt.Println("Start inizialization of server")

	serverIP := "0.0.0.0:50051"
	lis, err := net.Listen("tcp", serverIP)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer()
	blogpb.RegisterBlogServiceServer(s, &server{})

	// enable reflection for evans cli (test & show api)
	reflection.Register(s)

	go func() {
		fmt.Println("Server listen to:", serverIP)
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for Control C to exit
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	// Block until a signal is received
	<-ch

	fmt.Println("Closing mongodb connection")
	if err := clientDB.Disconnect(ctx); err != nil {
		log.Fatalf("error on disconnect with mongodb: %v", err)
	}

	fmt.Println("Closing the listener")
	if err := lis.Close(); err != nil {
		log.Fatalf("Error on closing the listener : %v", err)
	}
	fmt.Println("Stopping the server")
	s.Stop()
	fmt.Println("End of program")

}
