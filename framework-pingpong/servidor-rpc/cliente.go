package main 

import "net/rpc"

type Args struct {
	nombre string
}

type Reply struct {
	saludo string
}



func main(){
	var reply Reply
	args := Args{nombre: "Juan"}
	cliente , _ := rpc.Dial("tcp", "localhost:1234")
	cliente.Call("Servicio.Metodo", args, &reply)

}


