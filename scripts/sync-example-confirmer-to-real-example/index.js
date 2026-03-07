#!/usr/bin/env node

import fs from 'fs';
import yaml from 'yaml';

const {
	parseDocument,
	isMap,
	isSeq,
	isScalar,
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

		const indicesToRemove = [];

		for (let i = 0; i < stepsNode.items.length; i++) {
			const stepNode = stepsNode.items[i];
			const uses = stepNode.get('uses', true);

			if (uses?.value?.includes('actions/setup-go')) {
				indicesToRemove.push(i);
				continue;
			}
			const id = stepNode.get('id', true);

			if (id?.value === 'setup-mimosa') {
				stepNode.set('uses', 'hytromo/mimosa/gh/setup-action@v2-setup');
				stepNode.set('with', doc.createNode({ version: 'v0.1.2' }));
				stepNode.delete('run');
			}

			if (uses?.value?.includes('./gh/build-push-action')) {
				stepNode.set('uses', 'hytromo/mimosa/gh/build-push-action@v6-build-push');

				// Remove default values from the example (mimosa-setup-enabled: false since default is true).
				// Mutate in place to preserve value-level anchors and aliases.
				const withNode = stepNode.get('with', true);
				if (withNode && isMap(withNode)) {
					const idx = withNode.items.findIndex(
						(pair) =>
							isScalar(pair.key) &&
							pair.key.value === 'mimosa-setup-enabled' &&
							isScalar(pair.value) &&
							(pair.value.value === false || pair.value.value === 'false')
					);
					if (idx !== -1) {
						withNode.items.splice(idx, 1);
					}
					if (withNode.items.length === 0) {
						stepNode.delete('with');
					}
				}
			}
		}

		// Remove filtered steps in reverse order to preserve indices
		for (let j = indicesToRemove.length - 1; j >= 0; j--) {
			stepsNode.delete(indicesToRemove[j]);
		}
	}
} else {
	console.error(`❌ 'jobs' is not a YAML map`);
}

fs.writeFileSync(outputPath, doc.toString({ lineWidth: 0 }));
console.log(`✔ Wrote ${outputPath}`);
