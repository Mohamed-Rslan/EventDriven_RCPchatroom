package commons

/*
TODO
you may define any structs here to be used by the rpc
*/

// hint: you will need to have the server address fixed between clients and the coordinating server

type RegisterArgs struct {
    Nickname string
}


type MessageArgs struct {
    Nickname string
    Message  string
}
func Get_server_address() string {
	return "0.0.0.0:4488"
}
