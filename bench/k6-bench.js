import http from 'k6/http';
import { sleep, check } from 'k6';
// import { Trend } from 'k6/metrics';

// Custom metric for requests per second
// let reqs = new Trend('reqs_per_sec');

// WordPress base URL
const WP_URL = __ENV.WP_URL || 'http://127.0.0.1';

// Example pages (adjust according to your actual site)
const pages = [
	'/', // Home page

	// Static page
	'/sample-page/',
	'/lorem-ipsum/',

	// Category page
	'/category/uncategorized/',
	'/category/classic/',

	// Tag page
	'/tag/content-2/',
	'/tag/template/',

	// Post page
	'/1/hello-world/',
	'/993/template-excerpt-defined/',
	'/1788/block-image/',
	'/1785/block-button/',
	'/1784/block-cover/',
	'/1787/block-gallery/',
	'/1782/blocks-widgets/',
];
const pagesSz = pages.length;

// k6 options
export let options = {
	insecureSkipTLSVerify: true, // skip cert check

	vus: 50,                       // number of virtual users
	// duration: '2m',                // total test duration
	discardResponseBodies: true,   // reduce memory usage
	thresholds: {
		// 'reqs_per_sec': ['p(50)<500', 'p(90)<800', 'p(95)<1000', 'p(99)<1200', 'max<1500'],
		'http_req_duration': ['p(50)<500', 'p(90)<800', 'p(95)<1000', 'p(99)<1200', 'max<1500'],
	},
	stages: [
		{ duration: '10s', target: 50 },  // warm-up 10 seconds
		{ duration: '1m50s', target: 50 } // main test
	],
};

let i = 0;
export default function () {
	// force skip sidekick cache
	const cookieJar = http.cookieJar();
	cookieJar.set(WP_URL, 'wordpress_logged_in', '');

	// Randomly pick a page to request
	// const idx = Math.floor(Math.random() * pages.length);
	const idx = i++ % pagesSz;
	let page = pages[idx];
	const res = http.get(`${WP_URL}${page}`);

	check(res, {
		'status 200': (r) => r.status === 200
	});

	// Track requests per second
	// reqs.add(1 / res.timings.duration * 1000);

	// Simulate real user "think time"
	// sleep(Math.random() * 3 + 1);  // 1~4 seconds
}
