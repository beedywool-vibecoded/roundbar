const require_index = require("../utils/index.js");
// lib/rules/no-v-html.ts
/**
* @fileoverview Restrict or warn use of v-html to prevent XSS attack
* @author Nathan Zeplowitz
*/
var no_v_html_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "disallow use of v-html to prevent XSS attack",
			categories: ["vue3-recommended", "vue2-recommended"],
			url: "https://eslint.vuejs.org/rules/no-v-html.html"
		},
		fixable: null,
		schema: [{
			type: "object",
			properties: { ignorePattern: { type: "string" } },
			additionalProperties: false
		}],
		messages: { unexpected: "'v-html' directive can lead to XSS attack." }
	},
	create(context) {
		const options = context.options[0];
		const ignoreRegEx = options?.ignorePattern ? new RegExp(options.ignorePattern, "u") : void 0;
		/**
		* Check if the expression matches the ignore pattern
		*/
		function shouldIgnore(expression, sourceCode) {
			if (!ignoreRegEx || !expression) return false;
			if (expression.type === "Identifier") return ignoreRegEx.test(expression.name);
			const expressionText = sourceCode.getText(expression);
			return ignoreRegEx.test(expressionText);
		}
		return require_index.default.defineTemplateBodyVisitor(context, { "VAttribute[directive=true][key.name.name='html']"(node) {
			const sourceCode = context.sourceCode;
			if (sourceCode && node.value?.expression && shouldIgnore(node.value.expression, sourceCode)) return;
			context.report({
				node,
				loc: node.loc,
				messageId: "unexpected"
			});
		} });
	}
};
exports.default = no_v_html_default;
