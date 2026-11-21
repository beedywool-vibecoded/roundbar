const require_index = require("../utils/index.js");
// lib/rules/no-v-model-argument.ts
/**
* @author Przemyslaw Falowski (@przemkow)
* @fileoverview This rule checks whether v-model used on custom component do not have an argument
*/
var no_v_model_argument_default = {
	meta: {
		type: "problem",
		docs: {
			description: "disallow adding an argument to `v-model` used in custom component",
			categories: ["vue2-essential"],
			url: "https://eslint.vuejs.org/rules/no-v-model-argument.html"
		},
		fixable: null,
		deprecated: true,
		schema: [],
		messages: { vModelRequireNoArgument: "'v-model' directives require no argument." }
	},
	create(context) {
		return require_index.default.defineTemplateBodyVisitor(context, { "VAttribute[directive=true][key.name.name='model']"(node) {
			const element = node.parent.parent;
			if (node.key.argument && require_index.default.isCustomComponent(element)) context.report({
				node,
				loc: node.loc,
				messageId: "vModelRequireNoArgument"
			});
		} });
	}
};
exports.default = no_v_model_argument_default;
