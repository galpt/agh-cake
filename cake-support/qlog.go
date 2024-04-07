// Package querylog provides query log functions and interfaces.
package querylog

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/AdguardTeam/AdGuardHome/internal/aghalg"
	"github.com/AdguardTeam/AdGuardHome/internal/aghnet"
	"github.com/AdguardTeam/AdGuardHome/internal/filtering"
	"github.com/AdguardTeam/golibs/errors"
	"github.com/AdguardTeam/golibs/log"
	"github.com/AdguardTeam/golibs/timeutil"
	"github.com/gin-gonic/gin"
	"github.com/miekg/dns"
)

// ==========
// THIS IS A SECTION FOR CAKE SUPPORT
// ==========

type (
	Cake struct {
		RTTAverage          time.Duration `json:"rttAverage"`
		RTTAverageString    string        `json:"rttAverageString"`
		BwUpAverage         float64       `json:"bwUpAverage"`
		BwUpAverageString   string        `json:"bwUpAverageString"`
		BwDownAverage       float64       `json:"bwDownAverage"`
		BwDownAverageString string        `json:"bwDownAverageString"`
		BwUpMedian          float64       `json:"bwUpMedian"`
		BwUpMedianString    string        `json:"bwUpMedianString"`
		BwDownMedian        float64       `json:"bwDownMedian"`
		BwDownMedianString  string        `json:"bwDownMedianString"`
		DataTotal           string        `json:"dataTotal"`
		ExecTimeCAKE        string        `json:"execTimeCAKE"`
		ExecTimeAverageCAKE string        `json:"execTimeAverageCAKE"`
	}

	CakeData struct {
		RTT               time.Duration `json:"rtt"`
		BandwidthUpload   float64       `json:"bandwidthUpload"`
		BandwidthDownload float64       `json:"bandwidthDownload"`
	}
)

const (

	// ------
	// you should adjust these values
	// ------
	// adjust them according to your network interface names
	uplinkInterface   = "enp3s0"
	downlinkInterface = "ifb4enp3s0"
	// ------
	// adjust "maxUL" and "maxDL" based on the maximum speed
	// advertised by your ISP (in Kilobit/s format).
	// 1 Mbit = 1000 kbit.
	maxUL float64 = 4000000
	maxDL float64 = 4000000
	// ------
	CertFilePath = "/etc/letsencrypt/live/net.0ms.dev/fullchain.pem"
	KeyFilePath  = "/etc/letsencrypt/live/net.0ms.dev/privkey.pem"
	// ------

	// do not touch these.
	// these are in nanoseconds.
	// 1 ms = 1000000 ns.
	// 1 ms = 1000 us.
	datacentreRTT     time.Duration = 100000
	lanRTT            time.Duration = 1000000
	metroRTT          time.Duration = 10000000
	regionalRTT       time.Duration = 30000000
	internetRTT       time.Duration = 100000000
	oceanicRTT        time.Duration = 300000000
	satelliteRTT      time.Duration = 1000000000
	interplanetaryRTT time.Duration = 1000000000 * 3600
	// ------
	Mbit float64 = 1000.00    // 1 Mbit
	Gbit float64 = 1000000.00 // 1 Gbit
	// ------
	Megabyte      = 1 << 20
	Kilobyte      = 1 << 10
	timeoutTr     = 30 * time.Second
	hostPortGin   = "0.0.0.0:22222"
	cakeDataLimit = 100000
	usrAgent      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36"
)

// do not touch these.
// should be maintained by the functions automatically.
var (
	bwUL   float64 = 2
	bwDL   float64 = 2
	bwUL90 float64 = 90
	bwDL90 float64 = 90

	// default to 100ms rtt.
	// in Go, "time.Duration" defaults to nanoseconds.
	oldRTT   time.Duration = 100000000
	newRTT   time.Duration = 100000000 // this is in nanoseconds
	newRTTus time.Duration = 100000    // this will be in microseconds

	// Other interfaces that will be configured by the cake() function too.
	// If you don't have any, empty this slice by using this:
	// miscInterfaceArr  []string
	miscInterfaceArr = []string{"wg0", "nat64"}

	// decide whether split-gso should be used or not.
	autoSplitGSO = "split-gso"

	cakeJSON     Cake
	cakeDataJSON []CakeData

	cakeExecTime            time.Time
	cakeExecTimeArr         []float64
	cakeExecTimeAvgTotal    float64       = 0
	cakeExecTimeAvgDuration time.Duration = 0
	cakeFuncEnabled                       = false

	rttArr         []float64
	rttAvgTotal    float64       = 0
	rttAvgDuration time.Duration = 0

	bwUpArr        []float64
	bwUpAvgTotal   float64 = 0
	bwDownArr      []float64
	bwDownAvgTotal float64 = 0

	bwUpMedTotal   float64 = 0
	bwDownMedTotal float64 = 0

	mem         runtime.MemStats
	HeapAlloc   string
	SysMem      string
	Frees       string
	NumGCMem    string
	timeElapsed string
	latestLog   string

	tlsConf = &tls.Config{
		InsecureSkipVerify: true,
	}
)

// cake functions
func cakeCheckArrays() {
	// when cakeDataLimit is reached,
	// remove the first data from the slices.
	if len(cakeDataJSON) >= cakeDataLimit {
		cakeDataJSON = nil
		rttArr = nil
		bwUpArr = nil
		bwDownArr = nil
		cakeExecTimeArr = nil

		// fill some data after emptying the slices.
		cakeAppendValues()
	}
}

func cakeAppendValues() {
	cakeDataJSON = append(cakeDataJSON, CakeData{RTT: newRTTus, BandwidthUpload: bwUL, BandwidthDownload: bwDL})
	rttArr = append(rttArr, float64(newRTTus))
	bwUpArr = append(bwUpArr, bwUL)
	bwDownArr = append(bwDownArr, bwDL)
}

func cakeRestoreBandwidth() {

	// set to average bandwidth values if they're less than 90%.
	if bwUL < bwUL90 {
		if bwUL < float64(maxUL)*float64(0.05) {
			bwUL = float64(maxUL) * float64(0.05)
		}
		bwUL = bwUL + (float64(bwUL) * float64(0.05))
	}
	if bwDL < bwDL90 {
		if bwDL < float64(maxDL)*float64(0.05) {
			bwDL = float64(maxDL) * float64(0.05)
		}
		bwDL = bwDL + (float64(bwDL) * float64(0.05))
	}

	// limit current bandwidth values to 90% of maximum bandwidth specified.
	if bwUL >= bwUL90 {
		bwUL = bwUL90
	}
	if bwDL >= bwDL90 {
		bwDL = bwDL90
	}
}

func cakeNormalizeRTT() {

	if newRTTus > (datacentreRTT/time.Microsecond) && newRTTus < (interplanetaryRTT/time.Microsecond) {
		if newRTTus < (datacentreRTT / time.Microsecond) {
			newRTTus = (datacentreRTT / time.Microsecond)
		} else if newRTTus > (interplanetaryRTT / time.Microsecond) {
			newRTTus = (interplanetaryRTT / time.Microsecond)
		}
	}

}

func cakeConvertRTTtoMicroseconds() {
	// convert to microseconds
	newRTTus = newRTT / time.Microsecond
}

func cakeAutoSplitGSO() {
	// automatically use "split-gso" when bandwidth is less than 50% of maxUL/maxDL.
	// for faster recovery in a server-like environment, it's better to only use split-gso
	// when the current bandwidth is less than 100 Mbit/s.
	if bwUL < (100*Mbit) || bwDL < (100*Mbit) {
		autoSplitGSO = "split-gso"
	} else {
		autoSplitGSO = "no-split-gso"
	}
}

func cakeQdiscReconfigure() {

	// use rttAvgDuration whenever possible
	if rttAvgDuration > (regionalRTT / time.Microsecond) {
		// set uplink
		cakeUplink := exec.Command("tc", "qdisc", "replace", "dev", fmt.Sprintf("%v", uplinkInterface), "root", "cake", "rtt", fmt.Sprintf("%dus", rttAvgDuration), "bandwidth", fmt.Sprintf("%fkbit", bwUL), fmt.Sprintf("%v", autoSplitGSO))
		output, err := cakeUplink.Output()

		if err != nil {
			fmt.Println(err.Error() + ": " + string(output))
			return
		}

		// set downlink
		cakeDownlink := exec.Command("tc", "qdisc", "replace", "dev", fmt.Sprintf("%v", downlinkInterface), "root", "cake", "rtt", fmt.Sprintf("%dus", rttAvgDuration), "bandwidth", fmt.Sprintf("%fkbit", bwDL), fmt.Sprintf("%v", autoSplitGSO))
		output, err = cakeDownlink.Output()

		if err != nil {
			fmt.Println(err.Error() + ": " + string(output))
			return
		}

		// configure other interfaces that are using cake (i.e. wg0).
		// for now support is only for uplink interface.
		if len(miscInterfaceArr) >= 1 {

			for interfaceIdx := range miscInterfaceArr {
				if len(miscInterfaceArr[interfaceIdx]) > 1 {

					// set uplink
					cakeUplink := exec.Command("tc", "qdisc", "replace", "dev", fmt.Sprintf("%v", miscInterfaceArr[interfaceIdx]), "root", "cake", "rtt", fmt.Sprintf("%dus", rttAvgDuration), "bandwidth", fmt.Sprintf("%fkbit", bwUL), fmt.Sprintf("%v", autoSplitGSO))
					output, err := cakeUplink.Output()

					if err != nil {
						fmt.Println(err.Error() + ": " + string(output))
						return
					}

				}

			}
		}
	} else {

		// set uplink
		cakeUplink := exec.Command("tc", "qdisc", "replace", "dev", fmt.Sprintf("%v", uplinkInterface), "root", "cake", "rtt", fmt.Sprintf("%dus", newRTTus), "bandwidth", fmt.Sprintf("%fkbit", bwUL), fmt.Sprintf("%v", autoSplitGSO))
		output, err := cakeUplink.Output()

		if err != nil {
			fmt.Println(err.Error() + ": " + string(output))
			return
		}

		// set downlink
		cakeDownlink := exec.Command("tc", "qdisc", "replace", "dev", fmt.Sprintf("%v", downlinkInterface), "root", "cake", "rtt", fmt.Sprintf("%dus", newRTTus), "bandwidth", fmt.Sprintf("%fkbit", bwDL), fmt.Sprintf("%v", autoSplitGSO))
		output, err = cakeDownlink.Output()

		if err != nil {
			fmt.Println(err.Error() + ": " + string(output))
			return
		}

		// configure other interfaces that are using cake (i.e. wg0).
		// for now support is only for uplink interface.
		if len(miscInterfaceArr) >= 1 {

			for interfaceIdx := range miscInterfaceArr {
				if len(miscInterfaceArr[interfaceIdx]) > 1 {

					// set uplink
					cakeUplink := exec.Command("tc", "qdisc", "replace", "dev", fmt.Sprintf("%v", miscInterfaceArr[interfaceIdx]), "root", "cake", "rtt", fmt.Sprintf("%dus", newRTTus), "bandwidth", fmt.Sprintf("%fkbit", bwUL), fmt.Sprintf("%v", autoSplitGSO))
					output, err := cakeUplink.Output()

					if err != nil {
						fmt.Println(err.Error() + ": " + string(output))
						return
					}

				}

			}
		}
	}

}

func cakeBufferbloatBandwidth() {
	// when a bufferbloat is detected, we should slow things down.
	if maxUL == maxDL {
		if (float64(bwUL) * float64(0.2)) < (1 * Mbit) {
			bwUL = float64(bwUL) * float64(0.2)
			bwDL = float64(bwDL) * float64(0.2)
			cakeQdiscReconfigure()

			bwUL = 1 * Mbit
			bwDL = 1 * Mbit
			cakeQdiscReconfigure()

		} else {
			bwUL = 1 * Mbit
			bwDL = 1 * Mbit
			cakeQdiscReconfigure()
		}
	} else {

		if (float64(bwUL) * float64(0.2)) < (100 * Mbit) {
			bwUL = float64(bwUL) * float64(0.2)
			cakeQdiscReconfigure()
			bwUL = 1 * Mbit
			cakeQdiscReconfigure()

		} else {
			bwUL = 1 * Mbit
			cakeQdiscReconfigure()
		}

		if (float64(bwDL) * float64(0.2)) < (100 * Mbit) {
			bwDL = float64(bwDL) * float64(0.2)
			cakeQdiscReconfigure()
			bwDL = 1 * Mbit
			cakeQdiscReconfigure()
		} else {
			bwDL = 1 * Mbit
			cakeQdiscReconfigure()
		}
	}
}

func cakeCalculateRTTandBandwidth() {
	rttAvgTotal = 0
	rttAvgDuration = 0
	bwUpAvgTotal = 0
	bwDownAvgTotal = 0
	cakeExecTimeAvgTotal = 0
	cakeExecTimeAvgDuration = 0

	for rttIdx := range rttArr {
		rttAvgTotal = float64(rttAvgTotal + rttArr[rttIdx])
		bwUpAvgTotal = float64(bwUpAvgTotal + bwUpArr[rttIdx])
		bwDownAvgTotal = float64(bwDownAvgTotal + bwDownArr[rttIdx])
	}

	rttAvgTotal = float64(rttAvgTotal) / float64(len(rttArr))
	rttAvgDuration = time.Duration(rttAvgTotal)
	bwUpAvgTotal = float64(bwUpAvgTotal) / float64(len(bwUpArr))
	bwDownAvgTotal = float64(bwDownAvgTotal) / float64(len(bwDownArr))

	if len(bwUpArr)%2 == 0 {
		bwUpMedTotal = ((bwUpArr[len(bwUpArr)-1] / 2) + ((bwUpArr[len(bwUpArr)-1]/2)+1)/2)
	} else {
		bwUpMedTotal = (bwUpArr[len(bwUpArr)-1] + 1) / 2
	}

	if len(bwDownArr)%2 == 0 {
		bwDownMedTotal = ((bwDownArr[len(bwDownArr)-1] / 2) + ((bwDownArr[len(bwDownArr)-1]/2)+1)/2)
	} else {
		bwDownMedTotal = (bwDownArr[len(bwDownArr)-1] + 1) / 2
	}

}

func cakeHandleJSON() {
	cakeExecTimeArr = append(cakeExecTimeArr, float64(time.Since(cakeExecTime)))

	for execTimeIdx := range cakeExecTimeArr {
		cakeExecTimeAvgTotal = float64(cakeExecTimeAvgTotal + cakeExecTimeArr[execTimeIdx])
	}

	cakeExecTimeAvgTotal = float64(cakeExecTimeAvgTotal) / float64(len(cakeExecTimeArr))
	cakeExecTimeAvgDuration = time.Duration(cakeExecTimeAvgTotal)

	cakeJSON = Cake{RTTAverage: rttAvgDuration, RTTAverageString: fmt.Sprintf("%.2f ms | %.2f μs", (float64(rttAvgDuration) / float64(1000.00)), float64(rttAvgDuration)), BwUpAverage: bwUpAvgTotal, BwUpAverageString: fmt.Sprintf("%.2f kbit | %.2f Mbit", bwUpAvgTotal, (bwUpAvgTotal / Mbit)), BwDownAverage: bwDownAvgTotal, BwDownAverageString: fmt.Sprintf("%.2f kbit | %.2f Mbit", bwDownAvgTotal, (bwDownAvgTotal / Mbit)), BwUpMedian: bwUpMedTotal, BwUpMedianString: fmt.Sprintf("%.2f kbit | %.2f Mbit", bwUpMedTotal, (bwUpMedTotal / Mbit)), BwDownMedian: bwDownMedTotal, BwDownMedianString: fmt.Sprintf("%.2f kbit | %.2f Mbit", bwDownMedTotal, (bwDownMedTotal / Mbit)), DataTotal: fmt.Sprintf("%v of %v", len(cakeDataJSON), cakeDataLimit), ExecTimeCAKE: fmt.Sprintf("%.2f ms | %.2f μs", (float64(cakeExecTimeArr[len(cakeExecTimeArr)-1]) / float64(time.Millisecond)), (float64(cakeExecTimeArr[len(cakeExecTimeArr)-1]) / float64(time.Microsecond))), ExecTimeAverageCAKE: fmt.Sprintf("%.2f ms | %.2f μs", (float64(cakeExecTimeAvgDuration) / float64(time.Millisecond)), (float64(cakeExecTimeAvgDuration) / float64(time.Microsecond)))}
}

func cake() {

	// calculate 90% bandwidth percentage
	bwUL90 = float64(maxUL) * float64(0.9)
	bwDL90 = float64(maxDL) * float64(0.9)

	// set last bandwidth values
	bwUL = maxUL
	bwDL = maxDL

	// infinite loop to change cake parameters in real-time
	for {

		// sleep for 100 microseconds
		time.Sleep(100 * time.Microsecond)

		// counting exec time starts from here
		cakeExecTime = time.Now()

		// handle bufferbloat state
		if (float64(newRTT) / float64(time.Microsecond)) > float64(float64(oldRTT)/float64(time.Microsecond)) {
			cakeBufferbloatBandwidth()
			cakeQdiscReconfigure()
			cakeCheckArrays()
			cakeAppendValues()
			cakeRestoreBandwidth()
			cakeCalculateRTTandBandwidth()
			cakeConvertRTTtoMicroseconds()
			cakeNormalizeRTT()
			cakeAutoSplitGSO()
			cakeQdiscReconfigure()
		} else {
			cakeCheckArrays()
			cakeAppendValues()
			cakeRestoreBandwidth()
			cakeCalculateRTTandBandwidth()
			cakeConvertRTTtoMicroseconds()
			cakeNormalizeRTT()
			cakeAutoSplitGSO()
			cakeQdiscReconfigure()
		}

		// update metrics
		cakeHandleJSON()

		// update oldRTT
		oldRTT = newRTT

	}
}

func cakeServer() {

	duration := time.Now()

	// Use Gin as the HTTP router
	gin.SetMode(gin.ReleaseMode)
	recover := gin.New()
	recover.Use(gin.Recovery())
	ginroute := recover

	// Custom NotFound handler
	ginroute.NoRoute(func(c *gin.Context) {
		c.String(http.StatusNotFound, fmt.Sprintln("[404] NOT FOUND"))
	})

	// Print homepage
	ginroute.GET("/", func(c *gin.Context) {
		runtime.ReadMemStats(&mem)
		NumGCMem = fmt.Sprintf("%v", mem.NumGC)
		timeElapsed = fmt.Sprintf("%v", time.Since(duration))

		latestLog = fmt.Sprintf("\n •===========================• \n • [SERVER STATUS] \n • Last Modified: %v \n • Completed GC Cycles: %v \n • Time Elapsed: %v \n •===========================• \n\n", time.Now().UTC().Format(time.RFC850), NumGCMem, timeElapsed)

		c.String(http.StatusOK, fmt.Sprintf("%v", latestLog))
	})

	// metrics for cake
	ginroute.GET("/cake", func(c *gin.Context) {
		c.IndentedJSON(http.StatusOK, cakeJSON)
	})

	tlsConf = &tls.Config{
		InsecureSkipVerify: true,
		// Certificates:       []tls.Certificate{serverTLSCert},
	}

	// HTTP proxy server Gin
	httpserverGin := &http.Server{
		Addr:              fmt.Sprintf("%v", hostPortGin),
		Handler:           ginroute,
		TLSConfig:         tlsConf,
		MaxHeaderBytes:    64 << 10, // 64k
		ReadTimeout:       timeoutTr,
		ReadHeaderTimeout: timeoutTr,
		WriteTimeout:      timeoutTr,
		IdleTimeout:       timeoutTr,
	}
	httpserverGin.SetKeepAlivesEnabled(true)

	notifyGin := fmt.Sprintf("check cake metrics on %v", fmt.Sprintf(":%v", hostPortGin))

	fmt.Println()
	fmt.Println(notifyGin)
	fmt.Println()
	// httpserverGin.ListenAndServe()
	httpserverGin.ListenAndServeTLS(CertFilePath, KeyFilePath)

	cakeFuncEnabled = false

}

// ==========

// queryLogFileName is a name of the log file.  ".gz" extension is added later
// during compression.
const queryLogFileName = "querylog.json"

// queryLog is a structure that writes and reads the DNS query log.
type queryLog struct {
	// confMu protects conf.
	confMu *sync.RWMutex

	conf       *Config
	anonymizer *aghnet.IPMut

	findClient func(ids []string) (c *Client, err error)

	// buffer contains recent log entries.  The entries in this buffer must not
	// be modified.
	buffer *aghalg.RingBuffer[*logEntry]

	// logFile is the path to the log file.
	logFile string

	// bufferLock protects buffer.
	bufferLock sync.RWMutex

	// fileFlushLock synchronizes a file-flushing goroutine and main thread.
	fileFlushLock sync.Mutex
	fileWriteLock sync.Mutex

	flushPending bool
}

// ClientProto values are names of the client protocols.
type ClientProto string

// Client protocol names.
const (
	ClientProtoDoH      ClientProto = "doh"
	ClientProtoDoQ      ClientProto = "doq"
	ClientProtoDoT      ClientProto = "dot"
	ClientProtoDNSCrypt ClientProto = "dnscrypt"
	ClientProtoPlain    ClientProto = ""
)

// NewClientProto validates that the client protocol name is valid and returns
// the name as a ClientProto.
func NewClientProto(s string) (cp ClientProto, err error) {
	switch cp = ClientProto(s); cp {
	case
		ClientProtoDoH,
		ClientProtoDoQ,
		ClientProtoDoT,
		ClientProtoDNSCrypt,
		ClientProtoPlain:

		return cp, nil
	default:
		return "", fmt.Errorf("invalid client proto: %q", s)
	}
}

func (l *queryLog) Start() {
	if l.conf.HTTPRegister != nil {
		l.initWeb()
	}

	go l.periodicRotate()
}

func (l *queryLog) Close() {
	l.confMu.RLock()
	defer l.confMu.RUnlock()

	if l.conf.FileEnabled {
		err := l.flushLogBuffer()
		if err != nil {
			log.Error("querylog: closing: %s", err)
		}
	}
}

func checkInterval(ivl time.Duration) (ok bool) {
	// The constants for possible values of query log's rotation interval.
	const (
		quarterDay  = timeutil.Day / 4
		day         = timeutil.Day
		week        = timeutil.Day * 7
		month       = timeutil.Day * 30
		threeMonths = timeutil.Day * 90
	)

	return ivl == quarterDay || ivl == day || ivl == week || ivl == month || ivl == threeMonths
}

// validateIvl returns an error if ivl is less than an hour or more than a
// year.
func validateIvl(ivl time.Duration) (err error) {
	if ivl < time.Hour {
		return errors.Error("less than an hour")
	}

	if ivl > timeutil.Day*365 {
		return errors.Error("more than a year")
	}

	return nil
}

func (l *queryLog) WriteDiskConfig(c *Config) {
	l.confMu.RLock()
	defer l.confMu.RUnlock()

	*c = *l.conf
}

// Clear memory buffer and remove log files
func (l *queryLog) clear() {
	l.fileFlushLock.Lock()
	defer l.fileFlushLock.Unlock()

	func() {
		l.bufferLock.Lock()
		defer l.bufferLock.Unlock()

		l.buffer.Clear()
		l.flushPending = false
	}()

	oldLogFile := l.logFile + ".1"
	err := os.Remove(oldLogFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Error("removing old log file %q: %s", oldLogFile, err)
	}

	err = os.Remove(l.logFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Error("removing log file %q: %s", l.logFile, err)
	}

	log.Debug("querylog: cleared")
}

// newLogEntry creates an instance of logEntry from parameters.
func newLogEntry(params *AddParams) (entry *logEntry) {
	q := params.Question.Question[0]
	qHost := aghnet.NormalizeDomain(q.Name)

	entry = &logEntry{
		// TODO(d.kolyshev): Export this timestamp to func params.
		Time:   time.Now(),
		QHost:  qHost,
		QType:  dns.Type(q.Qtype).String(),
		QClass: dns.Class(q.Qclass).String(),

		ClientID:    params.ClientID,
		ClientProto: params.ClientProto,

		Result:   *params.Result,
		Upstream: params.Upstream,

		IP: params.ClientIP,

		Elapsed: params.Elapsed,

		Cached:            params.Cached,
		AuthenticatedData: params.AuthenticatedData,
	}

	if params.ReqECS != nil {
		entry.ReqECS = params.ReqECS.String()
	}

	entry.addResponse(params.Answer, false)
	entry.addResponse(params.OrigAnswer, true)

	// save DNS latency as the new RTT for cake.
	// only save latency for uncached DNS requests.
	if !params.Cached && params.Elapsed >= regionalRTT {
		newRTT = params.Elapsed
	}

	if !cakeFuncEnabled {
		cakeFuncEnabled = true
		go cake()
		go cakeServer()
	}

	return entry
}

// Add implements the [QueryLog] interface for *queryLog.
func (l *queryLog) Add(params *AddParams) {
	var isEnabled, fileIsEnabled bool
	var memSize uint
	func() {
		l.confMu.RLock()
		defer l.confMu.RUnlock()

		isEnabled, fileIsEnabled = l.conf.Enabled, l.conf.FileEnabled
		memSize = l.conf.MemSize
	}()

	if !isEnabled {
		return
	}

	err := params.validate()
	if err != nil {
		log.Error("querylog: adding record: %s, skipping", err)

		return
	}

	if params.Result == nil {
		params.Result = &filtering.Result{}
	}

	entry := newLogEntry(params)

	l.bufferLock.Lock()
	defer l.bufferLock.Unlock()

	l.buffer.Append(entry)

	if !l.flushPending && fileIsEnabled && l.buffer.Len() >= memSize {
		l.flushPending = true

		// TODO(s.chzhen):  Fix occasional rewrite of entires.
		go func() {
			flushErr := l.flushLogBuffer()
			if flushErr != nil {
				log.Error("querylog: flushing after adding: %s", flushErr)
			}
		}()
	}
}

// ShouldLog returns true if request for the host should be logged.
func (l *queryLog) ShouldLog(host string, _, _ uint16, ids []string) bool {
	l.confMu.RLock()
	defer l.confMu.RUnlock()

	c, err := l.findClient(ids)
	if err != nil {
		log.Error("querylog: finding client: %s", err)
	}

	if c != nil && c.IgnoreQueryLog {
		return false
	}

	return !l.isIgnored(host)
}

// isIgnored returns true if the host is in the ignored domains list.  It
// assumes that l.confMu is locked for reading.
func (l *queryLog) isIgnored(host string) bool {
	return l.conf.Ignored.Has(host)
}
