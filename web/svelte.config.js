import adapter from '@sveltejs/adapter-node';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	compilerOptions: {
		compatibility: {
			componentApi: 4
		}
	},
	kit: {
		adapter: adapter({
			out: 'build'
		})
	}
};

export default config;
