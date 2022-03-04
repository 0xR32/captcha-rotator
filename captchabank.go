package solver

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	d "bitbucket.org/babylonaio/pkg/datastore"
	"bitbucket.org/babylonaio/pkg/utils"
	astilectron "github.com/asticode/go-astilectron"
	atom "github.com/uber-go/atomic"
)

type CaptchaBank struct {
	ch            chan *Token
	manualCh      chan *Token
	done          chan bool
	stopProcess   chan bool
	stopFilter    chan bool
	pause         chan bool
	resume        chan bool
	clients       []Captcha
	w             *astilectron.Window
	counter       atom.Int32
	threadCount   atom.Int32
	manualCounter atom.Int32
	maxSize       int32
	queue         *utils.Queue
	Running       bool
	cancelFuncs   []context.CancelFunc
	cancelMu      *sync.Mutex
}

type Token struct {
	Token   string
	Created time.Time
	Type    string
}

var CaptchaB *CaptchaBank

func InitCaptchaBank(window *astilectron.Window) {
	CaptchaB = &CaptchaBank{
		ch:          make(chan *Token),
		manualCh:    make(chan *Token),
		done:        make(chan bool),
		stopProcess: make(chan bool),
		stopFilter:  make(chan bool),
		pause:       make(chan bool),
		resume:      make(chan bool),
		queue:       utils.NewQueue(),
		cancelFuncs: []context.CancelFunc{},
		cancelMu:    &sync.Mutex{},
		w:           window,
	}
}

func InitCaptchaBankClients(proxygroup string) {
	if d.DStore.Settings != nil {
		if d.DStore.Settings.Captcha != nil {
			CaptchaB.Clear()
			if d.DStore.Settings.Captcha.TwoCaptcha != "" {
				CaptchaB.AddCaptchaClient(InitTwoCaptcha(d.DStore.Settings.Captcha.TwoCaptcha))
			}
			if d.DStore.Settings.Captcha.AntiCaptcha != "" {
				CaptchaB.AddCaptchaClient(InitAntiCaptcha(d.DStore.Settings.Captcha.AntiCaptcha, proxygroup))
			}
			if d.DStore.Settings.Captcha.CapMonster != "" {
				CaptchaB.AddCaptchaClient(InitCapmonster(d.DStore.Settings.Captcha.CapMonster, proxygroup))
			}
		}
	}
}

func (c *CaptchaBank) SendSize() {
	m := map[string]int32{}
	m["api"] = c.counter.Load()
	m["manual"] = c.manualCounter.Load()
	payload := map[string]interface{}{
		"name":    "captchabank-tokens",
		"payload": m,
	}
	c.w.SendMessage(payload, func(m *astilectron.EventMessage) {
		return
	})
}

func (c *CaptchaBank) Empty() bool {
	return c.counter.Load() == 0 && c.manualCounter.Load() == 0
}

func (c *CaptchaBank) Push(t *Token) {
	c.queue.Append(t)
}

func (c *CaptchaBank) PushCancelFunc(f context.CancelFunc) {
	c.cancelMu.Lock()
	c.cancelFuncs = append(c.cancelFuncs, f)
	c.cancelMu.Unlock()
}

func (c *CaptchaBank) GetToken() string {
	if c.counter.Load() == 0 && c.manualCounter.Load() == 0 {
		return ""
	}
	for {
		t := c.queue.Pop()
		if t == nil {
			return ""
		}
		token := t.(*Token)
		if token != nil {
			if token.Type == "api" {
				c.counter.Dec()
			} else {
				c.manualCounter.Dec()
			}
			go c.SendSize()
			if time.Since(token.Created).Milliseconds() < 119000 {
				return token.Token
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

}

func (c *CaptchaBank) Filter() {
	for {
		select {
		case <-c.stopFilter:
			return
		default:
			if c.queue.Length() > 0 {
				i := c.queue.Length()
				for i > 0 {
					f := c.queue.Front()
					if f != nil {
						token := c.queue.Pop().(*Token)
						if time.Since(token.Created).Milliseconds() > 119000 {
							c.queue.Remove(f)
							if token.Type == "api" {
								c.counter.Dec()
							} else {
								c.manualCounter.Dec()
							}
						} else {
							c.Push(token)
						}
					}
					i--
				}
				go c.SendSize()
			}
			time.Sleep(4 * time.Second)
		}
	}
}

func (cb *CaptchaBank) AddCaptchaClient(c Captcha) {
	cb.clients = append(cb.clients, c)
}

func (cb *CaptchaBank) Clear() {
	cb.clients = []Captcha{}
}

func (c *CaptchaBank) Stop() {
	c.Running = false
	go func() {
		c.done <- true
		c.stopProcess <- true
		c.stopFilter <- true
	}()
}

func (c *CaptchaBank) Pause() {
	go func() {
		c.pause <- true
	}()
}

func (c *CaptchaBank) Resume() {
	go func() {
		c.resume <- true
	}()
}

func (c *CaptchaBank) Harvest(workers, maxSize float64) {
	c.maxSize = int32(maxSize)
	c.Running = true
	go c.ProcessTokens()
	for {
		select {
		case <-c.done:
			fmt.Println("finished")
			return
		case <-c.pause:
			fmt.Println("pausing")
			select {
			case <-c.resume:
				fmt.Println("resuming")
			case <-c.done:
				fmt.Println("finished")
				return
			}
		default:
			for i := 0; i < int(workers); i++ {
				go c.CreateTokenWithAPI()
				c.threadCount.Inc()
			}
			time.Sleep(2000 * time.Millisecond)
		}
	}
}

func (c *CaptchaBank) CreateToken(token string) {
	c.manualCounter.Inc()
	go c.SendSize()
	c.manualCh <- &Token{Token: token, Created: time.Now(), Type: "manual"}
}

func (c *CaptchaBank) CreateTokenWithAPI() {
	if c.counter.Load() >= c.maxSize {
		return
	}
	l := len(c.clients)
	if l == 0 {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.PushCancelFunc(cancel)

	r := rand.Intn(l)
	c.threadCount.Dec()
	t := c.clients[r].Solve(ctx)
	if t == "" || c.counter.Load() >= c.maxSize {
		return
	}
	c.counter.Inc()
	go c.SendSize()
	c.ch <- &Token{Token: t, Created: time.Now(), Type: "api"}
}

func (c *CaptchaBank) GetTokenWithAPI(ctx context.Context) string {
	l := len(c.clients)
	if l == 0 {
		return ""
	}

	r := rand.Intn(l)
	t := c.clients[r].Solve(ctx)
	if t == "" {
		return ""
	}
	return t
}

func (c *CaptchaBank) ProcessTokens() {
	go c.Filter()
	for {
		select {
		case <-c.stopProcess:
			c.cancelMu.Lock()
			for _, f := range c.cancelFuncs {
				go f()
			}
			c.cancelFuncs = []context.CancelFunc{}
			c.cancelMu.Unlock()
			return
		case t := <-c.ch:
			if time.Since(t.Created).Milliseconds() < 119000 {
				c.Push(t)
				continue
			}
			c.counter.Dec()
		case t := <-c.manualCh:
			if time.Since(t.Created).Milliseconds() < 119000 {
				c.Push(t)
				continue
			}
			c.manualCounter.Dec()
		default:
			continue
		}
	}
}
