const require_index = require("../utils/index.js");
// lib/rules/require-component-is.ts
/**
* @author Toru Nagashima
* @copyright 2017 Toru Nagashima. All rights reserved.
* See LICENSE file in root directory for full license.
*/
var require_component_is_default = {
	meta: {
		type: "problem",
		docs: {
			description: "require `v-bind:is` of `<component>` elements",
			categories: ["vue3-essential", "vue2-essential"],
			url: "https://eslint.vuejs.org/rules/require-component-is.html"
		},
		fixable: null,
		schema: [],
		messages: { requireComponentIs: "Expected '<component>' elements to have 'v-bind:is' attribute." }
	},
	create(context) {
		return require_index.default.defineTemplateBodyVisitor(context, { "VElement[name='component']"(node) {
			if (!require_index.default.hasDirective(node, "bind", "is")) context.report({
				node,
				loc: node.loc,
				messageId: "requireComponentIs"
			});
		} });
	}
};
exports.default = require_component_is_default;
