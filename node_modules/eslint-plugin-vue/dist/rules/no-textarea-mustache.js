const require_index = require("../utils/index.js");
// lib/rules/no-textarea-mustache.ts
/**
* @author Toru Nagashima
* @copyright 2017 Toru Nagashima. All rights reserved.
* See LICENSE file in root directory for full license.
*/
var no_textarea_mustache_default = {
	meta: {
		type: "problem",
		docs: {
			description: "disallow mustaches in `<textarea>`",
			categories: ["vue3-essential", "vue2-essential"],
			url: "https://eslint.vuejs.org/rules/no-textarea-mustache.html"
		},
		fixable: null,
		schema: [],
		messages: { unexpected: "Unexpected mustache. Use 'v-model' instead." }
	},
	create(context) {
		return require_index.default.defineTemplateBodyVisitor(context, { "VElement[rawName='textarea'] VExpressionContainer"(node) {
			if (node.parent.type !== "VElement") return;
			context.report({
				node,
				loc: node.loc,
				messageId: "unexpected"
			});
		} });
	}
};
exports.default = no_textarea_mustache_default;
