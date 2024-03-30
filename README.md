# agh-cake

> [!IMPORTANT]
>
> 1. This adaptation is using AdGuardHome to get DNS latency in real-time. You may want to visit the [DNSCrypt-CAKE](https://github.com/galpt/dnscrypt-cake) repository if you want to compare both tools.
> 2. If there is a new update from the [AdGuardHome](https://github.com/AdguardTeam/AdGuardHome) repository, you only need to replace the `qlog.go` file from the new version using the one with CAKE support and it will work just fine.
> 3. If there is a new update with an updated `qlog.go` from the official repository, we will try to update `qlog.go` (with CAKE support) accordingly.

> [!NOTE]
>
> The goal of this project is to provide another alternative that *"just works"* for not-so-technical users. Thus, users only need to set these values correctly: `uplinkInterface`, `downlinkInterface`, `maxDL`, and `maxUL`.

## Table of Contents

1. [About](https://github.com/galpt/agh-cake?tab=readme-ov-file#about)
2. [What to expect](https://github.com/galpt/agh-cake?tab=readme-ov-file#what-to-expect)
3. [Congestion Control Consideration](https://github.com/galpt/agh-cake?tab=readme-ov-file#congestion-control-consideration)
4. [How it works](https://github.com/galpt/agh-cake?tab=readme-ov-file#how-it-works)
5. [How to compile the code](https://github.com/galpt/agh-cake?tab=readme-ov-file#how-to-compile-the-code)
6. [See agh-cake in action](https://github.com/galpt/agh-cake?tab=readme-ov-file#see-agh-cake-in-action)
7. [Credits](https://github.com/galpt/agh-cake?tab=readme-ov-file#credits)

* * *

## About
#### [:arrow_up: Go to Table of Contents](https://github.com/galpt/agh-cake?tab=readme-ov-file#table-of-contents)

[CAKE (Common Applications Kept Enhanced)](https://www.bufferbloat.net/projects/codel/wiki/CakeTechnical/) is a comprehensive smart queue management that is available as a queue discipline (qdisc) for the Linux kernel. It is one of the best qdiscs designed to solve bufferbloat problems at the network edge.

According to the CAKE's [ROUND TRIP TIME PARAMETERS](https://man7.org/linux/man-pages/man8/tc-cake.8.html#ROUND_TRIP_TIME_PARAMETERS) man7 page, if there is a way to adjust the RTT dynamically in real-time, it should theoretically make CAKE able to give the best possible AQM results between latency and throughput.

`agh-cake` is an attempt to adjust CAKE's `rtt` parameter in real-time based on real latency per DNS request using a slightly modified version of [AdGuardHome](https://github.com/AdguardTeam/AdGuardHome). In addition to that, it will also adjust `bandwidth` intelligently while constantly monitoring your real RTT.

This is an adaptation of [dnscrypt-cake](https://github.com/galpt/dnscrypt-cake), inspired by the [cake-autorate](https://github.com/lynxthecat/cake-autorate) project, implemented in Go, but it's adjusting CAKE's `rtt` and `bandwidth` based on your every DNS request and what website you are visiting, not by only ping-ing to `1.1.1.1`, `8.8.8.8` and/or any other DNS servers.

This implementation is suitable for servers and networks where most of the users are actively sending DNS requests.

* * *

## What to expect
#### [:arrow_up: Go to Table of Contents](https://github.com/galpt/agh-cake?tab=readme-ov-file#table-of-contents)

There are several things you can expect from using this implementation:
1. You only need to worry about setting up `uplinkInterface`, `downlinkInterface`, `maxDL`, and `maxUL` correctly.
2. It will manage `bandwidth` intelligently (do a speedtest using [Speedtest CLI](https://www.speedtest.net/apps/cli) or similar tools to see it in action).
3. It will manage `rtt` ranging from 100us - 3600s. Unless your network is really that fast, you will see mostly 100ms RTT or higher and `agh-cake` will adjust CAKE's `rtt` accordingly.
4. It will manage `split-gso` automatically.
5. It is able to scale CAKE's `bandwidth` from 1 Mbit/s to 1 Gbit/s (or even more) in seconds.

> [!NOTE]
>
> Just set `maxDL` and `maxUL` based on whatever speed advertised by your ISP. No need to limit them to 90% or something like that. The code logic will try to handle that automatically.

* * *

## Congestion Control Consideration
#### [:arrow_up: Go to Table of Contents](https://github.com/galpt/agh-cake?tab=readme-ov-file#table-of-contents)

You may want to consider what TCP CC algorithm to use that works best for your workloads.
Different CC handles congestion differently, and that will affect how fast `agh-cake` is able to restore the configured bandwidth when a latency increase is detected.

Below are the CC algorithms that we have tested and worked well with `agh-cake` in a server environment:
1. `reno` — The Reno TCP CC
2. `cubic` — The [CUBIC](https://en.wikipedia.org/wiki/CUBIC_TCP) TCP CC
3. `scalable` — The [Scalable](https://en.wikipedia.org/wiki/Scalable_TCP) TCP CC
4. `dctcp` — The [DCTCP](https://datatracker.ietf.org/doc/html/rfc8257) TCP CC
5. `htcp` — The [H-TCP](https://en.wikipedia.org/wiki/H-TCP) TCP CC
6. `highspeed` — The [High Speed](https://en.wikipedia.org/wiki/HSTCP) TCP CC
7. `yeah` — The [YeAH](https://www.gdt.id.au/~gdt/presentations/2010-07-06-questnet-tcp/reference-materials/papers/baiocchi+castellani+vacirca-yeah-tcp-yet-another-highspeed-tcp.pdf) TCP CC
8. `bbr` — The [BBR](https://github.com/google/bbr) TCP CC (v1 and v3)

> [!IMPORTANT]
>
> 1. `dctcp` must not be deployed over the public Internet without additional measures.
> 2. Using `bbr` might cause issues such as frequent captchas on some websites or any other issues. This [article](https://blog.apnic.net/2020/01/10/when-to-use-and-not-use-bbr/) by APNIC can give you some references on when you may want to use it.

* * *

## How it works
#### [:arrow_up: Go to Table of Contents](https://github.com/galpt/agh-cake?tab=readme-ov-file#table-of-contents)

1. When a latency increase is detected, `agh-cake` will try to check if the DNS latency is in the range of 100us - 3600s or not.
If yes, then use that as CAKE's `rtt`, if not then use `rtt 100us` if it's less than 100us, and `rtt 3600s` if it's more than 3600s.
2. `agh-cake` will then adjust CAKE's `bandwidth` using all data in the `dataTotal` slice/array.
3. The `cake()` function will try to handle `bandwidth`, `rtt`, and `split-gso` in milliseconds.

> [!NOTE]
>
> The `cake()` function will configure CAKE and re-calculate `rtt` and `bandwidth`, then save the latest data into several slices/arrays. The arrays can hold up to 100000 data, and the `cake()` function will loop infinitely with a sleep of 100 microseconds for each loop. All data will be used to calculate the final values for configuring CAKE's `rtt` and `bandwidth`.
>
> This is an attempt to intelligently configure CAKE's `rtt` and `bandwidth` based on all the data, so it doesn't need to aggressively probe DNS servers like what the original [cake-autorate](https://github.com/lynxthecat/cake-autorate) implementation does.

* * *

## How to compile the code
#### [:arrow_up: Go to Table of Contents](https://github.com/galpt/agh-cake?tab=readme-ov-file#table-of-contents)

1. Download and install [The Go Programming Language](https://go.dev/).
2. Copy the files from `./agh-cake/cake-support` to `./agh-cake/AdGuardHome/internal/querylog`.
3. Edit the `qlog.go` file and adjust these values:
   1. `uplinkInterface` and `downlinkInterface` to your network interface names.
   2. `maxDL` and `maxUL` to your maximum network bandwidth (in kilobit/s format) advertised by your ISP.
   3. `CertFilePath` and `KeyFilePath` to where your SSL certificate is located.


4. Then, see the [How to build from source](https://github.com/AdguardTeam/AdGuardHome?tab=readme-ov-file#how-to-build) section to compile the code.

> [!IMPORTANT]
> 1. You have to run the binary with `sudo` since it needs to change the linux qdisc, so it needs enough permissions to do that.
> 2. It's not recommended to change `cakeUplink` and `cakeDownlink` parameters in the `qlog.go` file as they are intended to only handle `bandwidth` and `rtt`. If you need to change CAKE's parameters, change them directly from the terminal.
> 3. Use `httpserverGin.ListenAndServe()` instead of `httpserverGin.ListenAndServeTLS(CertFilePath, KeyFilePath)` in the `qlog.go` file if you don't want to use SSL certificate (i.e. you're using `localhost` instead of `0.0.0.0`).

* * *

## See `agh-cake` in action
#### [:arrow_up: Go to Table of Contents](https://github.com/galpt/agh-cake?tab=readme-ov-file#table-of-contents)

We are testing `agh-cake` in our server here:

https://net.0ms.dev:7777/netstat

See `agh-cake` metrics here:

https://net.0ms.dev:22222/cake

A quick speed/bufferbloat test using [Cloudflare Speed Test](https://speed.cloudflare.com/):

[Cloudflare Speed Test](https://cdn1.0ms.dev/1/cf.mp4)

A quick speed/bufferbloat test using [Waveform Speed Test](https://www.waveform.com/tools/bufferbloat):

[Waveform Speed Test](https://cdn1.0ms.dev/1/wvform.mp4)

> [!NOTE]
>
> Other details related to the tests:
>
> 1. The above tests were done via a VPN tunnel (WireGuard).
> 2. The above tests were using wireless connection.
> 3. Server location is in Singapore, and most users are located in Jakarta (Indonesia).

* * *

## Credits
#### [:arrow_up: Go to Table of Contents](https://github.com/galpt/agh-cake?tab=readme-ov-file#table-of-contents)

Although we are writing this guide to let people know about our implementation, it was made possible by using other things provided by the developers and/or companies mentioned in this guide.

All credits and copyrights go to the respective owners.
