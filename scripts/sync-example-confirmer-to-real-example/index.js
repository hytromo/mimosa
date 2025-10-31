#!/usr/bin/env node

import fs from 'fs';
import yaml from 'yaml';

const {
	parseDocument,
	isMap,
	isSeq,
} = yaml;

const inputPath = '.github/workflows/example-confirmer.yml';
const outputPath = 'docs/gh-actions/example-github-action.yml';

const source = fs.readFileSync(inputPath, 'utf8');
const doc = parseDocument(source, {
	keepCstNodes: true,
	keepNodeTypes: true,
	prettyErrors: true,
});

// Set workflow name
doc.set('name', 'Mimosa GitHub Action Example');

// Get jobs node
const jobs = doc.get('jobs', true);

if (isMap(jobs)) {
	for (const job of jobs.items) {
		const jobNode = job.value;

		const stepsNode = jobNode.get('steps', true);
		if (!isSeq(stepsNode)) continue;

		const newSteps = [];

		for (const stepNode of stepsNode.items) {
			const uses = stepNode.get('uses', true);

			if (uses?.value?.includes('actions/setup-go')) continue;
			const id = stepNode.get('id', true);

			if (id?.value === 'setup-mimosa') {
				stepNode.set('uses', 'hytromo/mimosa/gh/setup-action@v1-setup');
				stepNode.set('with', doc.createNode({ version: 'v0.1.0' }));
				stepNode.delete('run');

				newSteps.push(stepNode);
				continue;
			}

			if (uses?.value?.includes('./gh/cache-action')) {
				stepNode.set('uses', 'hytromo/mimosa/gh/cache-action@v2-cache');
			}

			newSteps.push(stepNode);
		}

		jobNode.set('steps', doc.createNode(newSteps));
	}
} else {
	console.error(`❌ 'jobs' is not a YAML map`);
}

fs.writeFileSync(outputPath, String(doc));
console.log(`✔ Wrote ${outputPath}`);
