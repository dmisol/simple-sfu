package defs

const (
	Addr    = ":50000"
	RamDisk = "/run/tmp"
)

type InitialJson struct {
	Dir  string      `json:"dir"`
	Ftar interface{} `json:"ftar"`

	Static string `json:"static,omitempty"`

	W   int `json:"width,omitempty"`
	H   int `json:"height,omitempty"`
	FPS int `json:"fps,omitempty"`

	HeadPos interface{} `json:"head_position,omitempty"`
	Tattoo  interface{} `json:"tattoo,omitempty"`
	Bkg     int         `json:"video_bkg"`

	Glasses interface{} `json:"glasses,omitempty"`
	Merge   int         `json:"merge_type"`
	Color   interface{} `json:"color_filter,omitempty"`
	Pi      interface{} `json:"pattern_index,omitempty"`

	Enc string `json:"out_encoding,omitempty"`

	//Batch_s int  `json:"batch_size,omitempty"`
	//Blur    bool `json:"motion_blur,omitempty"`
	//Hat     bool `json:"hat,omitempty"`
	//VR      bool `json:"vr,omitempty"`
	//HairSeg bool `json:"hair_seg,omitempty"`
}

type Anim struct {
	Ts int `json:"ts"` // milliseconds since start

	Audio   string    `json:"audio,omitempty"`   // file name with audio samples
	Phones  []*Viseme `json:"phones,omitempty"`  // phones like derived from vosk
	Pattern []float64 `json:"pattern,omitempty"` // model params
}

type Viseme struct {
	Time     int    `json:"time"` // ms
	Type     string `json:"type"`
	start    int
	Value    string `json:"value"`
	end      int
	Duration int `json:"duration"` // ms
}
