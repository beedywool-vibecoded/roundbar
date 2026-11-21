const require_index = require("../utils/index.js");
// lib/rules/require-prop-types.ts
var require_prop_types_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "require type definitions in props",
			categories: ["vue3-strongly-recommended", "vue2-strongly-recommended"],
			url: "https://eslint.vuejs.org/rules/require-prop-types.html"
		},
		fixable: null,
		schema: [],
		messages: { requireType: "Prop \"{{name}}\" should define at least its type." }
	},
	create(context) {
		function optionHasType(node) {
			switch (node.type) {
				case "ObjectExpression": return objectHasType(node);
				case "ArrayExpression": return node.elements.length > 0;
				case "FunctionExpression":
				case "ArrowFunctionExpression": return false;
			}
			return null;
		}
		function objectHasType(node) {
			const typeProperty = require_index.default.findProperty(node, "type", (p) => p.value.type !== "ArrayExpression" || p.value.elements.length > 0);
			const validatorProperty = require_index.default.findProperty(node, "validator");
			return Boolean(typeProperty || validatorProperty);
		}
		function checkProperty(prop) {
			if (prop.type !== "object" && prop.type !== "array") return;
			if (prop.type !== "array" && (optionHasType(prop.value) ?? true)) return;
			const { node, propName } = prop;
			const name = propName || node.type === "Identifier" && node.name || "Unknown prop";
			context.report({
				node,
				messageId: "requireType",
				data: { name }
			});
		}
		return require_index.default.compositingVisitors(require_index.default.defineScriptSetupVisitor(context, {
			onDefinePropsEnter(_node, props) {
				for (const prop of props) checkProperty(prop);
			},
			onDefineModelEnter(node, model) {
				if (model.typeNode || model.options && (optionHasType(model.options) ?? true)) return;
				context.report({
					node: model.options || node,
					messageId: "requireType",
					data: { name: model.name.modelName }
				});
			}
		}), require_index.default.executeOnVue(context, (obj) => {
			const props = require_index.default.getComponentPropsFromOptions(obj);
			for (const prop of props) checkProperty(prop);
		}));
	}
};
exports.default = require_prop_types_default;
