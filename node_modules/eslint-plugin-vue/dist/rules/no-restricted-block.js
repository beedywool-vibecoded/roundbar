const require_index = require("../utils/index.js");
const require_regexp = require("../utils/regexp.js");
// lib/rules/no-restricted-block.ts
/**
* @author Yosuke Ota
* See LICENSE file in root directory for full license.
*/
function parseOption(option) {
	if (typeof option === "string") {
		const matcher = require_regexp.toRegExp(option, { remove: "g" });
		return { test(block) {
			return matcher.test(block.rawName);
		} };
	}
	const parsed = parseOption(option.element);
	parsed.message = option.message;
	return parsed;
}
function defaultMessage(block) {
	return `Using \`<${block.rawName}>\` is not allowed.`;
}
var no_restricted_block_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "disallow specific block",
			categories: void 0,
			url: "https://eslint.vuejs.org/rules/no-restricted-block.html"
		},
		fixable: null,
		schema: {
			type: "array",
			items: { oneOf: [{ type: "string" }, {
				type: "object",
				properties: {
					element: { type: "string" },
					message: {
						type: "string",
						minLength: 1
					}
				},
				required: ["element"],
				additionalProperties: false
			}] },
			uniqueItems: true,
			minItems: 0
		},
		messages: { restrictedBlock: "{{message}}" }
	},
	create(context) {
		const options = context.options.map(parseOption);
		const sourceCode = context.sourceCode;
		const documentFragment = sourceCode.parserServices.getDocumentFragment && sourceCode.parserServices.getDocumentFragment();
		function getTopLevelHTMLElements() {
			return documentFragment ? documentFragment.children.filter(require_index.default.isVElement) : [];
		}
		return { Program(node) {
			if (require_index.default.hasInvalidEOF(node)) return;
			for (const block of getTopLevelHTMLElements()) {
				const option = options.find((option) => option.test(block));
				if (!option) continue;
				const message = option.message || defaultMessage(block);
				context.report({
					node: block.startTag,
					messageId: "restrictedBlock",
					data: { message }
				});
			}
		} };
	}
};
exports.default = no_restricted_block_default;
