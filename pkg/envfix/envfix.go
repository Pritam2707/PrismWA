package envfix

import "os"

func init() {
	// Prevent protobuf v2 registry conflict panic and silence duplicate warnings between whatsmeow and hypermeow (meowcaller)
	if os.Getenv("GOLANG_PROTOBUF_REGISTRATION_CONFLICT") == "" {
		_ = os.Setenv("GOLANG_PROTOBUF_REGISTRATION_CONFLICT", "ignore")
	}
}
