# Инкремент 17

##  Результат запусков 
```
>>> go test -bench=. -benchmem -run=^$  ./...
?   	github.com/rompil2/metrics_aggregator/cmd/agent	[no test files]
?   	github.com/rompil2/metrics_aggregator/cmd/server	[no test files]
PASS
ok  	github.com/rompil2/metrics_aggregator/internal/agent	0.391s
PASS
ok  	github.com/rompil2/metrics_aggregator/internal/audit	0.212s
PASS
ok  	github.com/rompil2/metrics_aggregator/internal/config	0.188s
{"level":"info","id":"testCounter","time":"2026-01-25T23:26:53+03:00","message":"metric created"}
goos: darwin
goarch: arm64
pkg: github.com/rompil2/metrics_aggregator/internal/handler
cpu: Apple M1 Pro
BenchmarkHandler_UpdateJSON-10           482778	      2397 ns/op	    7859 B/op	      38 allocs/op
BenchmarkHandler_GetMetricJSON-10    	 160238	      8040 ns/op	    7674 B/op	      33 allocs/op
PASS
ok  	github.com/rompil2/metrics_aggregator/internal/handler	3.640s
?   	github.com/rompil2/metrics_aggregator/internal/logger	[no test files]
?   	github.com/rompil2/metrics_aggregator/internal/mocks	[no test files]
?   	github.com/rompil2/metrics_aggregator/internal/model	[no test files]
goos: darwin
goarch: arm64
pkg: github.com/rompil2/metrics_aggregator/internal/repository
cpu: Apple M1 Pro
BenchmarkMemStore_Set-10       	14635603	        85.43 ns/op	      88 B/op	       3 allocs/op
BenchmarkMemStore_Get-10       	84225054	        14.18 ns/op	       0 B/op	       0 allocs/op
BenchmarkMemStore_GetAll-10    	27100270	        43.88 ns/op	      64 B/op	       1 allocs/op
PASS
ok  	github.com/rompil2/metrics_aggregator/internal/repository	3.930s
PASS
ok  	github.com/rompil2/metrics_aggregator/internal/repository/dbstore	0.284s
PASS
ok  	github.com/rompil2/metrics_aggregator/internal/repository/filestore	0.183s
PASS
ok  	github.com/rompil2/metrics_aggregator/internal/repository/memstore	0.183s
goos: darwin
goarch: arm64
pkg: github.com/rompil2/metrics_aggregator/internal/service
cpu: Apple M1 Pro
BenchmarkService_UpdateMetric-10    	13050487	        88.15 ns/op	      88 B/op	       3 allocs/op
BenchmarkService_GetMetric-10       	47490427	        24.95 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	github.com/rompil2/metrics_aggregator/internal/service	2.616s
?   	github.com/rompil2/metrics_aggregator/migrations	[no test files]
```

## Результат  pprof
```
>>> go tool pprof -top -diff_base=profiles/base.pprof profiles/result.pprof 2>/dev/null | head -30
File: main
Type: alloc_space
Time: 2026-01-23 22:29:56 MSK
Showing nodes accounting for -20011.21kB, 73.26% of 27314.72kB total
Dropped 1 node (cum <= 136.57kB)
      flat  flat%   sum%        cum   cum%
-4753.50kB 17.40% 17.40% -4753.50kB 17.40%  compress/flate.(*dictDecoder).init
-2570.08kB  9.41% 26.81% -2570.08kB  9.41%  reflect.growslice
-2112.67kB  7.73% 34.55% -3654.93kB 13.38%  io.copyBuffer
-1805.17kB  6.61% 41.16% -2354.01kB  8.62%  compress/flate.NewWriter
-1542.01kB  5.65% 46.80% -1542.01kB  5.65%  bufio.NewReaderSize
-1028.76kB  3.77% 50.57% -5782.26kB 21.17%  compress/flate.NewReader
-1026.25kB  3.76% 54.32% -1026.25kB  3.76%  bytes.growSlice
-1024.06kB  3.75% 58.07% -1536.09kB  5.62%  github.com/rompil2/metrics_aggregator/internal/repository/memstore.(*MemStorage).SetMetrics
-1024.05kB  3.75% 61.82% -1024.05kB  3.75%  internal/sync.newEntryNode[go.shape.interface {},go.shape.interface {}]
 -548.84kB  2.01% 63.83%  -548.84kB  2.01%  compress/flate.(*compressor).initDeflate
 -521.05kB  1.91% 65.74%  -521.05kB  1.91%  runtime.itabsinit
 -516.01kB  1.89% 67.63%  -516.01kB  1.89%  io.init.func1
     514kB  1.88% 65.75%      514kB  1.88%  encoding/json.(*Decoder).refill
    -514kB  1.88% 67.63%     -514kB  1.88%  encoding/json.appendIndent
    -514kB  1.88% 69.51%     -514kB  1.88%  runtime/pprof.writeHeapInternal
 -512.69kB  1.88% 71.39%  -512.69kB  1.88%  sync.(*Pool).pinSlow
 -512.06kB  1.87% 73.26% -4104.24kB 15.03%  github.com/rompil2/metrics_aggregator/internal/handler.(*HandlerMux).UpdatesWithJSON
 -512.05kB  1.87% 75.14%  -512.05kB  1.87%  time.NewTicker
  512.05kB  1.87% 73.26%   512.05kB  1.87%  runtime.(*scavengerState).init
 -512.01kB  1.87% 75.14%  -512.01kB  1.87%  crypto/internal/fips140hash.UnwrapNew[go.shape.interface { BlockSize int; Reset; Size int; Sum []uint8; Write  }]
  512.01kB  1.87% 73.26%   512.01kB  1.87%  encoding/json.(*decodeState).literalStore
         0     0% 73.26% -1542.01kB  5.65%  bufio.NewReader
         0     0% 73.26% -1026.25kB  3.76%  bytes.(*Buffer).Write
         0     0% 73.26% -1026.25kB  3.76%  bytes.(*Buffer).grow

```

