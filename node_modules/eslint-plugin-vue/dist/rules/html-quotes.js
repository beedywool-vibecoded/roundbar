const require_index = require("../utils/index.js");
// lib/rules/html-quotes.ts
/**
* @author Toru Nagashima
* @copyright 2017 Toru Nagashima. All rights reserved.
* See LICENSE file in root directory for full license.
*/
var html_quotes_default = {
	meta: {
		type: "layout",
		docs: {
			description: "enforce quotes style of HTML attributes",
			categories: ["vue3-strongly-recommended", "vue2-strongly-recommended"],
			url: "https://eslint.vuejs.org/rules/html-quotes.html"
		},
		fixable: "code",
		schema: [{ enum: ["double", "single"] }, {
			type: "object",
			properties: { avoidEscape: { type: "boolean" } },
			additionalProperties: false
		}],
		messages: { expected: "Expected to be enclosed by {{kind}}." }
	},
	create(context) {
		const sourceCode = context.sourceCode;
		const shouldUseDoubleQuotes = context.options[0] !== "single";
		const avoidEscape = context.options[1] && context.options[1].avoidEscape === true;
		const quoteChar = shouldUseDoubleQuotes ? "\"" : "'";
		const quoteName = shouldUseDoubleQuotes ? "double quotes" : "single quotes";
		let hasInvalidEOF;
		return require_index.default.defineTemplateBodyVisitor(context, { "VAttribute[value!=null]"(node) {
			if (hasInvalidEOF || require_index.default.isVBindSameNameShorthand(node)) return;
			const text = sourceCode.getText(node.value);
			const firstChar = text[0];
			if (firstChar === quoteChar) return;
			const isQuoted = firstChar === "'" || firstChar === "\"";
			if (avoidEscape && isQuoted) {
				if (text.slice(1, -1).includes(quoteChar)) return;
			}
			context.report({
				node: node.value,
				loc: node.value.loc,
				messageId: "expected",
				data: { kind: quoteName },
				fix(fixer) {
					const contentText = isQuoted ? text.slice(1, -1) : text;
					let shouldFixToDouble = shouldUseDoubleQuotes;
					if (avoidEscape && !isQuoted && contentText.includes(quoteChar)) shouldFixToDouble = shouldUseDoubleQuotes ? contentText.includes("'") : !contentText.includes("\"");
					const quotePattern = shouldFixToDouble ? /"/g : /'/g;
					const quoteEscaped = shouldFixToDouble ? "&quot;" : "&apos;";
					const fixQuoteChar = shouldFixToDouble ? "\"" : "'";
					const replacement = fixQuoteChar + contentText.replace(quotePattern, () => quoteEscaped) + fixQuoteChar;
					return fixer.replaceText(node.value, replacement);
				}
			});
		} }, { Program(node) {
			hasInvalidEOF = require_index.default.hasInvalidEOF(node);
		} });
	}
};
exports.default = html_quotes_default;
