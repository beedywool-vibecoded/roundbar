const require_index = require("../utils/index.js");
// lib/rules/valid-v-pre.ts
/**
* @author Toru Nagashima
* @copyright 2017 Toru Nagashima. All rights reserved.
* See LICENSE file in root directory for full license.
*/
var valid_v_pre_default = {
	meta: {
		type: "problem",
		docs: {
			description: "enforce valid `v-pre` directives",
			categories: ["vue3-essential", "vue2-essential"],
			url: "https://eslint.vuejs.org/rules/valid-v-pre.html"
		},
		fixable: null,
		schema: [],
		messages: {
			unexpectedArgument: "'v-pre' directives require no argument.",
			unexpectedModifier: "'v-pre' directives require no modifier.",
			unexpectedValue: "'v-pre' directives require no attribute value."
		}
	},
	create(context) {
		return require_index.default.defineTemplateBodyVisitor(context, { "VAttribute[directive=true][key.name.name='pre']"(node) {
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
			if (node.value) context.report({
				node: node.value,
				messageId: "unexpectedValue"
			});
		} });
	}
};
exports.default = valid_v_pre_default;
