const require_index = require("../utils/index.js");
// lib/rules/one-component-per-file.ts
/**
* @fileoverview enforce that each component should be in its own file
* @author Armano
*/
var one_component_per_file_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "enforce that each component should be in its own file",
			categories: ["vue3-strongly-recommended", "vue2-strongly-recommended"],
			url: "https://eslint.vuejs.org/rules/one-component-per-file.html"
		},
		fixable: null,
		schema: [],
		messages: { tooManyComponents: "There is more than one component in this file." }
	},
	create(context) {
		const components = [];
		return Object.assign({}, require_index.default.executeOnVueComponent(context, (node, type) => {
			if (type === "definition") {
				if (require_index.default.getVueComponentDefinitionType(context, node) === "mixin") return;
			}
			components.push(node);
		}), { "Program:exit"() {
			if (components.length > 1) for (const node of components) context.report({
				node,
				messageId: "tooManyComponents"
			});
		} });
	}
};
exports.default = one_component_per_file_default;
