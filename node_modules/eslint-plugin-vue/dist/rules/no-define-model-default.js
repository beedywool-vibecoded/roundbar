const require_index = require("../utils/index.js");
// lib/rules/no-define-model-default.ts
/**
* @author Valentin Yushkevich
* See LICENSE file in root directory for full license.
*/
var no_define_model_default_default = {
	meta: {
		type: "problem",
		docs: {
			description: "disallow default values in `defineModel`",
			categories: void 0,
			url: "https://eslint.vuejs.org/rules/no-define-model-default.html"
		},
		fixable: null,
		schema: [],
		messages: { unexpected: "Unexpected default value for the \"{{name}}\" model." }
	},
	create(context) {
		return require_index.default.defineScriptSetupVisitor(context, { onDefineModelEnter(_node, model) {
			const options = model.options && require_index.default.skipTSAsExpression(model.options);
			if (options?.type !== "ObjectExpression") return;
			const defaultProperty = require_index.default.findProperty(options, "default");
			if (defaultProperty) context.report({
				node: defaultProperty,
				messageId: "unexpected",
				data: { name: model.name.modelName }
			});
		} });
	}
};
exports.default = no_define_model_default_default;
