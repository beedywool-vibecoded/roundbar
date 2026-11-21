const require_index = require("../utils/index.js");
// lib/rules/valid-v-html.ts
/**
* @author Toru Nagashima
* @copyright 2017 Toru Nagashima. All rights reserved.
* See LICENSE file in root directory for full license.
*/
var valid_v_html_default = {
	meta: {
		type: "problem",
		docs: {
			description: "enforce valid `v-html` directives",
			categories: ["vue3-essential", "vue2-essential"],
			url: "https://eslint.vuejs.org/rules/valid-v-html.html"
		},
		fixable: null,
		schema: [],
		messages: {
			unexpectedArgument: "'v-html' directives require no argument.",
			unexpectedModifier: "'v-html' directives require no modifier.",
			expectedValue: "'v-html' directives require that attribute value."
		}
	},
	create(context) {
		return require_index.default.defineTemplateBodyVisitor(context, { "VAttribute[directive=true][key.name.name='html']"(node) {
			if (node.key.argument) context.report({
				node: node.key.argument,
				messageId: "unexpectedArgument"
			});
			const lastModifier = node.key.modifiers.at(-1);
			if (lastModifier) context.report({
				node,
				loc: {
					start: node.key.modifiers[0].loc.start,
					end: lastModifier.loc.end
				},
				messageId: "unexpectedModifier"
			});
			if (!node.value || require_index.default.isEmptyValueDirective(node, context)) context.report({
				node,
				messageId: "expectedValue"
			});
		} });
	}
};
exports.default = valid_v_html_default;
