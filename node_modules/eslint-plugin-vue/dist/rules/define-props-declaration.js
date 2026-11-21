const require_index = require("../utils/index.js");
// lib/rules/define-props-declaration.ts
/**
* @author Amorites
* See LICENSE file in root directory for full license.
*/
var define_props_declaration_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "enforce declaration style of `defineProps`",
			categories: void 0,
			url: "https://eslint.vuejs.org/rules/define-props-declaration.html"
		},
		fixable: null,
		schema: [{ enum: ["type-based", "runtime"] }],
		messages: {
			hasArg: "Use type-based declaration instead of runtime declaration.",
			hasTypeArg: "Use runtime declaration instead of type-based declaration."
		}
	},
	create(context) {
		const scriptSetup = require_index.default.getScriptSetupElement(context);
		if (!scriptSetup || !require_index.default.hasAttribute(scriptSetup, "lang", "ts")) return {};
		const defineType = context.options[0] || "type-based";
		return require_index.default.defineScriptSetupVisitor(context, { onDefinePropsEnter(node) {
			switch (defineType) {
				case "type-based":
					if (node.arguments.length > 0) context.report({
						node,
						messageId: "hasArg"
					});
					break;
				case "runtime": {
					const typeArguments = "typeArguments" in node ? node.typeArguments : node.typeParameters;
					if (typeArguments && typeArguments.params.length > 0) context.report({
						node,
						messageId: "hasTypeArg"
					});
					break;
				}
			}
		} });
	}
};
exports.default = define_props_declaration_default;
