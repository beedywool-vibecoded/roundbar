const require_index = require("../utils/index.js");
// lib/rules/no-deprecated-data-object-declaration.ts
/**
* @fileoverview disallow using deprecated object declaration on data
* @author yoyo930021
*/
function isOpenParen(token) {
	return token.type === "Punctuator" && token.value === "(";
}
function isCloseParen(token) {
	return token.type === "Punctuator" && token.value === ")";
}
function getFirstAndLastTokens(node, sourceCode) {
	let first = sourceCode.getFirstToken(node);
	let last = sourceCode.getLastToken(node);
	while (true) {
		const prev = sourceCode.getTokenBefore(first);
		const next = sourceCode.getTokenAfter(last);
		if (isOpenParen(prev) && isCloseParen(next)) {
			first = prev;
			last = next;
		} else return {
			first,
			last
		};
	}
}
var no_deprecated_data_object_declaration_default = {
	meta: {
		type: "problem",
		docs: {
			description: "disallow using deprecated object declaration on data (in Vue.js 3.0.0+)",
			categories: ["vue3-essential"],
			url: "https://eslint.vuejs.org/rules/no-deprecated-data-object-declaration.html"
		},
		fixable: "code",
		schema: [],
		messages: { objectDeclarationIsDeprecated: "Object declaration on 'data' property is deprecated. Using function declaration instead." }
	},
	create(context) {
		const sourceCode = context.sourceCode;
		const validPropertyTypes = /* @__PURE__ */ new Set([
			"FunctionExpression",
			"ArrowFunctionExpression",
			"Identifier"
		]);
		return require_index.default.executeOnVue(context, (objectExpression) => {
			const invalidData = require_index.default.findProperty(objectExpression, "data", (property) => !validPropertyTypes.has(property.value.type));
			if (invalidData) context.report({
				node: invalidData,
				messageId: "objectDeclarationIsDeprecated",
				fix(fixer) {
					const tokens = getFirstAndLastTokens(invalidData.value, sourceCode);
					return [fixer.insertTextBefore(tokens.first, "function() {\nreturn "), fixer.insertTextAfter(tokens.last, ";\n}")];
				}
			});
		});
	}
};
exports.default = no_deprecated_data_object_declaration_default;
