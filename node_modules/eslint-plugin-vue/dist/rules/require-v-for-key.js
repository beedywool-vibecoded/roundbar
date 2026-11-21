const require_index = require("../utils/index.js");
// lib/rules/require-v-for-key.ts
/**
* @author Toru Nagashima
* @copyright 2017 Toru Nagashima. All rights reserved.
* See LICENSE file in root directory for full license.
*/
var require_v_for_key_default = {
	meta: {
		type: "problem",
		docs: {
			description: "require `v-bind:key` with `v-for` directives",
			categories: ["vue3-essential", "vue2-essential"],
			url: "https://eslint.vuejs.org/rules/require-v-for-key.html"
		},
		fixable: null,
		schema: [],
		messages: { requireKey: "Elements in iteration expect to have 'v-bind:key' directives." }
	},
	create(context) {
		/**
		* Check the given element about `v-bind:key` attributes.
		*/
		function checkKey(element) {
			if (require_index.default.hasDirective(element, "bind", "key")) return;
			if (element.name === "template" || element.name === "slot") {
				for (const child of element.children) if (child.type === "VElement") checkKey(child);
			} else if (!require_index.default.isCustomComponent(element)) context.report({
				node: element.startTag,
				loc: element.startTag.loc,
				messageId: "requireKey"
			});
		}
		return require_index.default.defineTemplateBodyVisitor(context, { "VAttribute[directive=true][key.name.name='for']"(node) {
			checkKey(node.parent.parent);
		} });
	}
};
exports.default = require_v_for_key_default;
