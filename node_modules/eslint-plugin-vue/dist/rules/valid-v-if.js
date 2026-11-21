const require_index = require("../utils/index.js");
// lib/rules/valid-v-if.ts
/**
* @author Toru Nagashima
* @copyright 2017 Toru Nagashima. All rights reserved.
* See LICENSE file in root directory for full license.
*/
var valid_v_if_default = {
	meta: {
		type: "problem",
		docs: {
			description: "enforce valid `v-if` directives",
			categories: ["vue3-essential", "vue2-essential"],
			url: "https://eslint.vuejs.org/rules/valid-v-if.html"
		},
		fixable: null,
		schema: [],
		messages: {
			withVElse: "'v-if' and 'v-else' directives can't exist on the same element. You may want 'v-else-if' directives.",
			withVElseIf: "'v-if' and 'v-else-if' directives can't exist on the same element.",
			unexpectedArgument: "'v-if' directives require no argument.",
			unexpectedModifier: "'v-if' directives require no modifier.",
			expectedValue: "'v-if' directives require that attribute value."
		}
	},
	create(context) {
		return require_index.default.defineTemplateBodyVisitor(context, { "VAttribute[directive=true][key.name.name='if']"(node) {
			const element = node.parent.parent;
			if (require_index.default.hasDirective(element, "else")) context.report({
				node,
				messageId: "withVElse"
			});
			if (require_index.default.hasDirective(element, "else-if")) context.report({
				node,
				messageId: "withVElseIf"
			});
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
exports.default = valid_v_if_default;
