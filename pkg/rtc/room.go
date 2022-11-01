package rtc

type Room struct {
	Users map[string]*User // by [id]
}
