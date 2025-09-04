# simple benchmark

* test condition
	* wordpress v6.8.2
	* no page cahe, no object cache
	* plugins:
		* performance-lab: active
		* wordpress-importer: inactive
	* test data and plugin should same as swissspidy/compare-wp-performance
		* https://github.com/swissspidy/compare-wp-performance/blob/main/.github/workflows/benchmark.yml#L118-L128
	* http response compression:
		* all set to no compression
		* may not be realistic, suggestion?
* hardware spec:
	* CPU: AMD Ryzen 3 PRO 4350G with Radeon Graphics (4C8T)
	* RAM: DDR4-3200 32G x2 + 16G x2
	* SSD: Kingston A2000 NVMe SSD 1TB



## benchmark result

### frankenwp
* http response compression supported: br zstd gzip
```
$ bash run-bench.sh frankenwp.001.html 

         /\      Grafana   /‾‾/  
    /\  /  \     |\  __   /  /   
   /  \/    \    | |/ /  /   ‾‾\ 
  /          \   |   (  |  (‾)  |
 / __________ \  |_|\_\  \_____/ 

     execution: local
        script: k6-bench.js
 web dashboard: http://127.0.0.1:5665
        output: -

     scenarios: (100.00%) 1 scenario, 50 max VUs, 2m30s max duration (incl. graceful stop):
              * default: Up to 50 looping VUs for 2m0s over 2 stages (gracefulRampDown: 30s, gracefulStop: 30s)



  █ THRESHOLDS 

    http_req_duration
    ✗ 'p(50)<500' p(50)=564.25ms
    ✗ 'p(90)<800' p(90)=1.23s
    ✗ 'p(95)<1000' p(95)=1.38s
    ✗ 'p(99)<1200' p(99)=1.69s
    ✗ 'max<1500' max=3.94s


  █ TOTAL RESULTS 

    checks_total.......: 9111    75.319381/s
    checks_succeeded...: 100.00% 9111 out of 9111
    checks_failed......: 0.00%   0 out of 9111

    ✓ status 200

    HTTP
    http_req_duration..............: avg=660.07ms min=215.27ms med=564.25ms max=3.94s p(90)=1.23s p(95)=1.38s
      { expected_response:true }...: avg=660.07ms min=215.27ms med=564.25ms max=3.94s p(90)=1.23s p(95)=1.38s
    http_req_failed................: 0.00%  0 out of 9111
    http_reqs......................: 9111   75.319381/s

    EXECUTION
    iteration_duration.............: avg=660.34ms min=215.5ms  med=564.53ms max=3.94s p(90)=1.23s p(95)=1.38s
    iterations.....................: 9111   75.319381/s
    vus............................: 50     min=50        max=50
    vus_max........................: 50     min=50        max=50

    NETWORK
    data_received..................: 804 MB 6.6 MB/s
    data_sent......................: 1.0 MB 8.4 kB/s




running (2m01.0s), 00/50 VUs, 9111 complete and 0 interrupted iterations
default ✓ [======================================] 00/50 VUs  2m0s
ERRO[0121] thresholds on metrics 'http_req_duration' have been crossed
```


### caddy + php-fpm
* http response compression supported: zstd gzip
```
$ bash run-bench.sh caddy.php-fpm.001.html 

         /\      Grafana   /‾‾/  
    /\  /  \     |\  __   /  /   
   /  \/    \    | |/ /  /   ‾‾\ 
  /          \   |   (  |  (‾)  |
 / __________ \  |_|\_\  \_____/ 

     execution: local
        script: k6-bench.js
 web dashboard: http://127.0.0.1:5665
        output: -

     scenarios: (100.00%) 1 scenario, 50 max VUs, 2m30s max duration (incl. graceful stop):
              * default: Up to 50 looping VUs for 2m0s over 2 stages (gracefulRampDown: 30s, gracefulStop: 30s)



  █ THRESHOLDS 

    http_req_duration
    ✗ 'p(50)<500' p(50)=590.3ms
    ✗ 'p(90)<800' p(90)=1.4s
    ✗ 'p(95)<1000' p(95)=1.73s
    ✗ 'p(99)<1200' p(99)=2.34s
    ✗ 'max<1500' max=2.97s


  █ TOTAL RESULTS 

    checks_total.......: 8648    71.554035/s
    checks_succeeded...: 100.00% 8648 out of 8648
    checks_failed......: 0.00%   0 out of 8648

    ✓ status 200

    HTTP
    http_req_duration..............: avg=694.71ms min=55.48ms med=590.3ms  max=2.97s p(90)=1.4s p(95)=1.73s
      { expected_response:true }...: avg=694.71ms min=55.48ms med=590.3ms  max=2.97s p(90)=1.4s p(95)=1.73s
    http_req_failed................: 0.00%  0 out of 8648
    http_reqs......................: 8648   71.554035/s

    EXECUTION
    iteration_duration.............: avg=695.02ms min=55.75ms med=590.54ms max=2.97s p(90)=1.4s p(95)=1.74s
    iterations.....................: 8648   71.554035/s
    vus............................: 50     min=50        max=50
    vus_max........................: 50     min=50        max=50

    NETWORK
    data_received..................: 763 MB 6.3 MB/s
    data_sent......................: 960 kB 7.9 kB/s




running (2m00.9s), 00/50 VUs, 8648 complete and 0 interrupted iterations
default ✓ [======================================] 00/50 VUs  2m0s
ERRO[0121] thresholds on metrics 'http_req_duration' have been crossed
```


### nginx + php-fpm
* http response compression supported: gzip
```
$ bash run-bench.sh nginx.php-fpm.001.html

         /\      Grafana   /‾‾/  
    /\  /  \     |\  __   /  /   
   /  \/    \    | |/ /  /   ‾‾\ 
  /          \   |   (  |  (‾)  |
 / __________ \  |_|\_\  \_____/ 

     execution: local
        script: k6-bench.js
 web dashboard: http://127.0.0.1:5665
        output: -

     scenarios: (100.00%) 1 scenario, 50 max VUs, 2m30s max duration (incl. graceful stop):
              * default: Up to 50 looping VUs for 2m0s over 2 stages (gracefulRampDown: 30s, gracefulStop: 30s)



  █ THRESHOLDS 

    http_req_duration
    ✗ 'p(50)<500' p(50)=619.05ms
    ✗ 'p(90)<800' p(90)=1.41s
    ✗ 'p(95)<1000' p(95)=1.8s
    ✗ 'p(99)<1200' p(99)=2.43s
    ✗ 'max<1500' max=3.11s


  █ TOTAL RESULTS 

    checks_total.......: 8317    68.772917/s
    checks_succeeded...: 100.00% 8317 out of 8317
    checks_failed......: 0.00%   0 out of 8317

    ✓ status 200

    HTTP
    http_req_duration..............: avg=722.59ms min=60.94ms med=619.05ms max=3.11s p(90)=1.41s p(95)=1.8s
      { expected_response:true }...: avg=722.59ms min=60.94ms med=619.05ms max=3.11s p(90)=1.41s p(95)=1.8s
    http_req_failed................: 0.00%  0 out of 8317
    http_reqs......................: 8317   68.772917/s

    EXECUTION
    iteration_duration.............: avg=722.93ms min=61.2ms  med=619.38ms max=3.11s p(90)=1.41s p(95)=1.8s
    iterations.....................: 8317   68.772917/s
    vus............................: 50     min=50        max=50
    vus_max........................: 50     min=50        max=50

    NETWORK
    data_received..................: 734 MB 6.1 MB/s
    data_sent......................: 923 kB 7.6 kB/s




running (2m00.9s), 00/50 VUs, 8317 complete and 0 interrupted iterations
default ✓ [======================================] 00/50 VUs  2m0s
ERRO[0121] thresholds on metrics 'http_req_duration' have been crossed
```


### apache2
* http response compression supported: gzip
```
$ bash run-bench.sh apache.001.html

         /\      Grafana   /‾‾/  
    /\  /  \     |\  __   /  /   
   /  \/    \    | |/ /  /   ‾‾\ 
  /          \   |   (  |  (‾)  |
 / __________ \  |_|\_\  \_____/ 

     execution: local
        script: k6-bench.js
 web dashboard: http://127.0.0.1:5665
        output: -

     scenarios: (100.00%) 1 scenario, 50 max VUs, 2m30s max duration (incl. graceful stop):
              * default: Up to 50 looping VUs for 2m0s over 2 stages (gracefulRampDown: 30s, gracefulStop: 30s)



  █ THRESHOLDS 

    http_req_duration
    ✗ 'p(50)<500' p(50)=580.06ms
    ✗ 'p(90)<800' p(90)=1.45s
    ✗ 'p(95)<1000' p(95)=1.85s
    ✗ 'p(99)<1200' p(99)=2.52s
    ✗ 'max<1500' max=8.51s


  █ TOTAL RESULTS 

    checks_total.......: 8495    70.299783/s
    checks_succeeded...: 100.00% 8495 out of 8495
    checks_failed......: 0.00%   0 out of 8495

    ✓ status 200

    HTTP
    http_req_duration..............: avg=707.53ms min=36.45ms med=580.06ms max=8.51s p(90)=1.45s p(95)=1.85s
      { expected_response:true }...: avg=707.53ms min=36.45ms med=580.06ms max=8.51s p(90)=1.45s p(95)=1.85s
    http_req_failed................: 0.00%  0 out of 8495
    http_reqs......................: 8495   70.299783/s

    EXECUTION
    iteration_duration.............: avg=707.84ms min=36.62ms med=580.38ms max=8.51s p(90)=1.45s p(95)=1.85s
    iterations.....................: 8495   70.299783/s
    vus............................: 50     min=50        max=50
    vus_max........................: 50     min=50        max=50

    NETWORK
    data_received..................: 749 MB 6.2 MB/s
    data_sent......................: 943 kB 7.8 kB/s




running (2m00.8s), 00/50 VUs, 8495 complete and 0 interrupted iterations
default ✓ [======================================] 00/50 VUs  2m0s
ERRO[0121] thresholds on metrics 'http_req_duration' have been crossed
```

### haproxy + php-fpm (+ caddy)
* haproxy use lua check backend
	* only static file serve from caddy
	* *.php or any not-exist => php-fpm
* http response compression supported: br zstd gzip
```
$ bash run-bench.sh haproxy.001.html 

         /\      Grafana   /‾‾/  
    /\  /  \     |\  __   /  /   
   /  \/    \    | |/ /  /   ‾‾\ 
  /          \   |   (  |  (‾)  |
 / __________ \  |_|\_\  \_____/ 

     execution: local
        script: k6-bench.js
 web dashboard: http://127.0.0.1:5665
        output: -

     scenarios: (100.00%) 1 scenario, 50 max VUs, 2m30s max duration (incl. graceful stop):
              * default: Up to 50 looping VUs for 2m0s over 2 stages (gracefulRampDown: 30s, gracefulStop: 30s)



  █ THRESHOLDS 

    http_req_duration
    ✗ 'p(50)<500' p(50)=589.93ms
    ✗ 'p(90)<800' p(90)=1.41s
    ✗ 'p(95)<1000' p(95)=1.79s
    ✗ 'p(99)<1200' p(99)=2.29s
    ✗ 'max<1500' max=4.35s


  █ TOTAL RESULTS 

    checks_total.......: 8659    71.553456/s
    checks_succeeded...: 100.00% 8659 out of 8659
    checks_failed......: 0.00%   0 out of 8659

    ✓ status 200

    HTTP
    http_req_duration..............: avg=694.49ms min=31.55ms med=589.93ms max=4.35s p(90)=1.41s p(95)=1.79s
      { expected_response:true }...: avg=694.49ms min=31.55ms med=589.93ms max=4.35s p(90)=1.41s p(95)=1.79s
    http_req_failed................: 0.00%  0 out of 8659
    http_reqs......................: 8659   71.553456/s

    EXECUTION
    iteration_duration.............: avg=694.82ms min=31.74ms med=590.27ms max=4.35s p(90)=1.41s p(95)=1.79s
    iterations.....................: 8659   71.553456/s
    vus............................: 3      min=3         max=50
    vus_max........................: 50     min=50        max=50

    NETWORK
    data_received..................: 763 MB 6.3 MB/s
    data_sent......................: 961 kB 7.9 kB/s




running (2m01.0s), 00/50 VUs, 8659 complete and 0 interrupted iterations
default ✓ [======================================] 00/50 VUs  2m0s
ERRO[0121] thresholds on metrics 'http_req_duration' have been crossed
```
