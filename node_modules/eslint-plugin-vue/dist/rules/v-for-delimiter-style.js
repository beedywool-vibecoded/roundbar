const require_index = require("../utils/index.js");
// lib/rules/v-for-delimiter-style.ts
/**
* @fileoverview enforce `v-for` directive's delimiter style
* @author Flo Edelmann
* @copyright 2020 Flo Edelmann. All rights reserved.
* See LICENSE file in root directory for full license.
*/
var v_for_delimiter_style_default = {
	meta: {
		type: "layout",
		docs: {
			description: "enforce `v-for` directive's delimiter style",
			categories: void 0,
			url: "https://eslint.vuejs.org/rules/v-for-delimiter-style.html"
		},
		fixable: "code",
		schema: [{ enum: ["in", "of"] }],
		messages: { expected: "Expected '{{preferredDelimiter}}' instead of '{{usedDelimiter}}' in 'v-for'." }
	},
	create(context) {
		const preferredDelimiter = context.options[0] || "in";
		return require_index.default.defineTemplateBodyVisitor(context, { VForExpression(node) {
			const sourceCode = context.sourceCode;
			const tokenStore = sourceCode.parserServices.getTemplateBodyTokenStore && sourceCode.parserServices.getTemplateBodyTokenStore();
			const lastLeftNode = node.left.at(-1);
			const delimiterToken = tokenStore.getTokenAfter(lastLeftNode ?? tokenStore.getFirstToken(node), (token) => token.type !== "Punctuator");
			if (delimiterToken.value === preferredDelimiter) return;
			context.report({
				node,
				loc: node.loc,
				messageId: "expected",
				data: {
					preferredDelimiter,
					usedDelimiter: delimiterToken.value
				},
				*fix(fixer) {
					yield fixer.replaceText(delimiterToken, preferredDelimiter);
				}
			});
		} });
	}
};
exports.default = v_for_delimiter_style_default;
