const require_index = require("../utils/index.js");
// lib/rules/no-content-outside-block.ts
/**
* @author Valentin Yushkevich
* See LICENSE file in root directory for full license.
*/
var no_content_outside_block_default = {
	meta: {
		type: "problem",
		docs: {
			description: "disallow content outside of the top-level blocks",
			categories: void 0,
			url: "https://eslint.vuejs.org/rules/no-content-outside-block.html"
		},
		fixable: null,
		hasSuggestions: true,
		schema: [],
		messages: {
			unexpectedContent: "Unexpected content outside of the top-level blocks. It is dropped by the compiler.",
			removeContent: "Remove the content outside of the top-level blocks."
		}
	},
	create(context) {
		const sourceCode = context.sourceCode;
		const documentFragment = sourceCode.parserServices.getDocumentFragment && sourceCode.parserServices.getDocumentFragment();
		if (!documentFragment) return {};
		return { Program(node) {
			if (require_index.default.hasInvalidEOF(node)) return;
			for (const child of documentFragment.children) {
				if (child.type !== "VText") continue;
				const [start, end] = child.range;
				const raw = sourceCode.text.slice(start, end);
				const leading = raw.length - raw.trimStart().length;
				if (leading === raw.length) continue;
				const trailing = raw.length - raw.trimEnd().length;
				const contentRange = [start + leading, end - trailing];
				context.report({
					loc: {
						start: sourceCode.getLocFromIndex(contentRange[0]),
						end: sourceCode.getLocFromIndex(contentRange[1])
					},
					messageId: "unexpectedContent",
					suggest: [{
						messageId: "removeContent",
						fix: (fixer) => fixer.removeRange(contentRange)
					}]
				});
			}
		} };
	}
};
exports.default = no_content_outside_block_default;
