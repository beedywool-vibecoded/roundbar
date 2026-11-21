const require_index = require("../utils/index.js");
// lib/rules/define-props-destructuring.ts
/**
* @author Wayne Zhang
* See LICENSE file in root directory for full license.
*/
var define_props_destructuring_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "enforce consistent style for props destructuring",
			categories: void 0,
			url: "https://eslint.vuejs.org/rules/define-props-destructuring.html"
		},
		fixable: null,
		schema: [{
			type: "object",
			properties: { destructure: { enum: [
				"only-when-assigned",
				"always",
				"never"
			] } },
			additionalProperties: false
		}],
		messages: {
			preferDestructuring: "Prefer destructuring from `defineProps` directly.",
			avoidDestructuring: "Avoid destructuring from `defineProps`.",
			avoidWithDefaults: "Avoid using `withDefaults` with destructuring."
		}
	},
	create(context) {
		const destructurePreference = (context.options[0] || {}).destructure || "only-when-assigned";
		return require_index.default.compositingVisitors(require_index.default.defineScriptSetupVisitor(context, { onDefinePropsEnter(node, props) {
			if (!props.some((prop) => prop.propName || prop.type === "unknown" && prop.node)) return;
			const hasDestructure = require_index.default.isUsingPropsDestructure(node);
			const hasWithDefaults = require_index.default.hasWithDefaults(node);
			const hasAssigned = require_index.default.getLeftOfDefineProps(node) !== null;
			if (destructurePreference === "never") {
				if (hasDestructure) context.report({
					node,
					messageId: "avoidDestructuring"
				});
				return;
			}
			if (!hasDestructure && (destructurePreference === "always" || hasAssigned)) {
				context.report({
					node,
					messageId: "preferDestructuring"
				});
				return;
			}
			if (hasWithDefaults) context.report({
				node: node.parent.callee,
				messageId: "avoidWithDefaults"
			});
		} }));
	}
};
exports.default = define_props_destructuring_default;
