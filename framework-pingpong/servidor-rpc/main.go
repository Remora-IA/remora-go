package main 
import "net/rpc"


type Args struct {
	nombre string
}

type Reply struct {
	saludo string
}

type Servicio struct {}


func main(){


	servicio := Servicio{}


	_ = rpc.Register(&servicio)


}