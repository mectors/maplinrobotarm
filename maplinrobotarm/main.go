// maplinrobotarm project main.go
package main

import (
	"github.com/fromkeith/gorest"
	"encoding/json"
	"fmt"
	"github.com/kylelemons/gousb/usb"
	"net/http"
	"time"
)

type RobotPos struct {
	Grips    int
	Wrist    int
	Elbow    int
	Shoulder int
	Base     int
	Led      int
}

const (
	gripsmax    = 3
	wristmax    = 20
	elbowmax    = 64
	shouldermax = 60
	basemax     = 100
)

var robotpos RobotPos
var loopstop bool

func main() {
	robotpos = RobotPos{Grips: 3, Wrist: 10, Elbow: 32, Shoulder: 30, Base: 50, Led: 0}
	fmt.Println("Robot Postion:", robotpos)
	gorest.RegisterService(new(RobotService))
	http.Handle("/", gorest.Handle())
	fmt.Println("Listening on 8787")
	http.ListenAndServe(":8787", nil)
}

type RobotService struct {
	// Service level config
	gorest.RestService `root:"/robot/" consumes:"application/json" produces:"application/json"`
	// call /robot/move/{grips 0,1,2}/{wrist:0,1,2}/{elbow 0,1,2}/{shoulder 0,1,2}/{base 0,1,2}/{led 0,2}/{testmax true,fals}
	move gorest.EndPoint `method:"GET" path:"/move/{Grips:int}/{Wrist:int}/{Elbow:int}/{Shoulder:int}/{Base:int}/{Led:int}/{Testmax:bool}" output:"string"`
	// call /robot/set/{grips 0-3}/{wrist 0-50}/{elbow 0-50}/{shoulder 0-50}/{base 0-50}
	set gorest.EndPoint `method:"GET" path:"/set/{Grips:int}/{Wrist:int}/{Elbow:int}/{Shoulder:int}/{Base:int}/{Led:int}" output:"string"`
	// call /robot/moveto/{grips 0-3}/{wrist 0-50}/{elbow 0-50}/{shoulder 0-60}/{base 0-120}/{led 0,2}
	moveto gorest.EndPoint `method:"GET" path:"/moveto/{Grips:int}/{Wrist:int}/{Elbow:int}/{Shoulder:int}/{Base:int}/{Led:int}" output:"string"`
	// call /robot/play/{sleep: seconds}/{loop: true/false} moves contain array of robotpos and sleeps between the steps
	play gorest.EndPoint `method:"POST" path:"/play/{Sleep:int}/{Loop:bool}" postdata:"string"`
	// call /robot/endloop
	endloop gorest.EndPoint `method:"GET" path:"/endloop" output:"string"`
}

func (serv RobotService) Play(moves string, sleep int, loop bool) {
	fmt.Println("Play:", moves)
	byt := []byte(moves)
	dat := []RobotPos{}
	err := json.Unmarshal(byt, &dat)

	if err != nil {
		fmt.Println("Error in JSON", err)
		serv.ResponseBuilder().SetResponseCode(404).Overide(true) //Overide causes the entity returned by the method to be ignored. Other wise it would send back zeroed object
		return
	}

	loopstop = false
	for !loopstop {
		for _, move := range dat {
			serv.Moveto(move.Grips, move.Wrist, move.Elbow, move.Shoulder, move.Base, move.Led)
		}
		if sleep > 0 {
			fmt.Println("Sleeping:", sleep)
			time.Sleep(time.Duration(sleep) * time.Second)
		}
		fmt.Println("Loop:", loop)
		if !loop {
			loopstop = true
		}
		if loopstop {
			serv.Moveto(3, 10, 32, 30, 50, 0)
		}
	}

	return
}

func (serv RobotService) Endloop() (code string) {
	loopstop = true
	return
}

func (serv RobotService) Move(Grips int, Wrist int, Elbow int, Shoulder int, Base int, Led int, Testmax bool) (code string) {
	fmt.Println("Move", Grips, Wrist, Elbow, Shoulder, Base, Led, Testmax)
	rcode, err := Do(byte(GetArmCode(Grips, Wrist, Elbow, Shoulder, Testmax)), byte(GetBaseCode(Base, Testmax)), byte(GetLedCode(Led)))
	if err != nil {
		serv.ResponseBuilder().SetResponseCode(404).Overide(true) //Overide causes the entity returned by the method to be ignored. Other wise it would send back zeroed object
		return
	}
	code = string(rcode)
	return
}

func (serv RobotService) Set(Grips int, Wrist int, Elbow int, Shoulder int, Base int, Led int) (code string) {
	fmt.Println("Set", Grips, Wrist, Elbow, Shoulder, Base)
	robotpos.Base = Base
	robotpos.Elbow = Elbow
	robotpos.Grips = Grips
	robotpos.Shoulder = Shoulder
	robotpos.Wrist = Wrist
	robotpos.Led = Led
	code = "0"
	return
}

// we know where the robot arm is from robotpos, now we need to calculate how many times
// we need to move each element to get to the desired final position and move it there
func (serv RobotService) Moveto(Grips int, Wrist int, Elbow int, Shoulder int, Base int, Led int) (code string) {
	fmt.Println("MoveTo", Grips, Wrist, Elbow, Shoulder, Base, Led)
	movmt := []int{robotpos.Grips - Grips, robotpos.Wrist - Wrist, robotpos.Elbow - Elbow, robotpos.Shoulder - Shoulder, robotpos.Base - Base}
	max := Abs(movmt[0]) // assume first value is the smallest
	for _, value := range movmt {
		if Abs(value) > max {
			max = Abs(value) // found another smaller value, replace previous value in max
		}
	}
	fmt.Println("max value", max)
	for i := 0; i < max; i++ {
		serv.Move(NeedMove(&movmt[0]),
			NeedMove(&movmt[1]),
			NeedMove(&movmt[2]),
			NeedMove(&movmt[3]),
			NeedMove(&movmt[4]),
			Led,
			true)
	}
	code = "0"
	return
}

// Abs returns the absolute value of x.
func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// how many positive or negative moves remain
// return 0 if no more movement is required, 1 if negative movement, 2 if postive
func NeedMove(remain *int) (movement int) {
	switch {
	case *remain < 0:
		*remain++
		fmt.Println("remain++", *remain)
		return 2
	case *remain > 0:
		*remain--
		fmt.Println("remain--", *remain)
		return 1
	default:
		fmt.Println("remain00", *remain)
		return 0
	}
}

func GetLedCode(Led int) (code int) {
	switch {
	case Led >= 1:
		Led = 1
	default:
		Led = 0
	}
	return Led
}

func GetBaseCode(Base int, testmax bool) (code int) {
	switch {
	case Base >= 2:
		if testmax && robotpos.Base+1 > basemax {
			fmt.Println("Reached max of base")
			Base = 0
		} else {
			if testmax {
				robotpos.Base = robotpos.Base + 1
			}
			Base = 2
		}
	case Base == 1:
		if testmax && robotpos.Base-1 < 0 {
			fmt.Println("Reached min of base")
			Base = 0
		} else {
			if testmax {
				robotpos.Base = robotpos.Base - 1
			}
			Base = 1
		}
	default:
		Base = 0
	}
	return Base
}

func GetArmCode(Grips int, Wrist int, Elbow int, Shoulder int, testmax bool) (code int) {
	switch {
	case Grips >= 2:
		if testmax && robotpos.Grips+1 > gripsmax {
			fmt.Println("Reached max of grips")
			Grips = 0
		} else {
			if testmax {
				robotpos.Grips = robotpos.Grips + 1
			}
			Grips = 2
		}
	case Grips == 1:
		if testmax && robotpos.Grips-1 < 0 {
			fmt.Println("Reached min of grips")
			Grips = 0
		} else {
			if testmax {
				robotpos.Grips = robotpos.Grips - 1
			}
			Grips = 1
		}
	default:
		Grips = 0
	}
	switch {
	case Wrist >= 2:
		if testmax && robotpos.Wrist+1 > wristmax {
			fmt.Println("Reached max of wrist")
			Wrist = 0
		} else {
			if testmax {
				robotpos.Wrist = robotpos.Wrist + 1
			}
			Wrist = 4
		}
	case Wrist == 1:
		if testmax && robotpos.Wrist-1 < 0 {
			fmt.Println("Reached min of wrist")
			Wrist = 0
		} else {
			if testmax {
				robotpos.Wrist = robotpos.Wrist - 1
			}
			Wrist = 8
		}
	default:
		Wrist = 0
	}
	switch {
	case Elbow >= 2:
		if testmax && robotpos.Elbow+1 > elbowmax {
			fmt.Println("Reached max of elbow")
			Elbow = 0
		} else {
			if testmax {
				robotpos.Elbow = robotpos.Elbow + 1
			}
			Elbow = 16
		}
	case Elbow == 1:
		if testmax && robotpos.Elbow-1 < 0 {
			fmt.Println("Reached min of elbow")
			Elbow = 0
		} else {
			if testmax {
				robotpos.Elbow = robotpos.Elbow - 1
			}
			Elbow = 32
		}
	default:
		Elbow = 0
	}
	switch {
	case Shoulder >= 2:
		if testmax && robotpos.Shoulder+1 > shouldermax {
			fmt.Println("Reached max of shoulder")
			Shoulder = 0
		} else {
			if testmax {
				robotpos.Shoulder = robotpos.Shoulder + 1
			}
			Shoulder = 64
		}
	case Shoulder == 1:
		if testmax && robotpos.Shoulder-1 < 0 {
			fmt.Println("Reached min of shoulder")
			Shoulder = 0
		} else {
			if testmax {
				robotpos.Shoulder = robotpos.Shoulder - 1
			}
			Shoulder = 128
		}
	default:
		Shoulder = 0
	}
	code = Grips + Wrist + Elbow + Shoulder
	fmt.Println("Robot Pos:", robotpos)
	fmt.Println("Code:", code)
	return code
}

func Do(arm byte, base byte, led byte) (int, error) {
	command := []byte{arm, base, led}
	context := usb.NewContext()
	defer context.Close()
	var robotDesc *usb.Descriptor
	devs, err := context.ListDevices(func(desc *usb.Descriptor) bool {
		if desc.Vendor == 0x1267 {
			robotDesc = desc
			return true
		}
		return false
	})
	defer func() {
		for _, d := range devs {
			d.Close()
		}
	}()
	if robotDesc == nil {
		fmt.Println("Could not find Maplin Robot Arm")
		var reterr error
		return 0, reterr
	}

	if err != nil {
		fmt.Println("Some devices had an error: %s", err)
		var reterr error
		return 0, reterr
	}
	devs[0].Control(0x40, 6, 0x100, 0, command)
	time.Sleep(150 * time.Millisecond)
	return devs[0].Control(0x40, 6, 0x100, 0, []byte{0, 0, led})
}
