const require_index = require("../utils/index.js");
// lib/rules/no-required-prop-with-default.ts
/**
* @author @neferqiqi
* See LICENSE file in root directory for full license.
*/
var no_required_prop_with_default_default = {
	meta: {
		type: "problem",
		docs: {
			description: "enforce props with default values to be optional",
			categories: ["vue2-recommended", "vue3-recommended"],
			url: "https://eslint.vuejs.org/rules/no-required-prop-with-default.html"
		},
		fixable: "code",
		hasSuggestions: true,
		schema: [{
			type: "object",
			properties: { autofix: { type: "boolean" } },
			additionalProperties: false
		}],
		messages: {
			requireOptional: `Prop "{{ key }}" should be optional.`,
			fixRequiredProp: `Change this prop to be optional.`
		}
	},
	create(context) {
		let canAutoFix = false;
		const option = context.options[0];
		if (option) canAutoFix = option.autofix;
		const handleObjectProp = (prop, defaultProps) => {
			if (prop.type === "object" && prop.propName && prop.value.type === "ObjectExpression" && (require_index.default.findProperty(prop.value, "default") || defaultProps?.has(prop.propName))) {
				const requiredProperty = require_index.default.findProperty(prop.value, "required");
				if (!requiredProperty) return;
				const requiredNode = requiredProperty.value;
				if (requiredNode && requiredNode.type === "Literal" && Boolean(requiredNode.value)) context.report({
					node: prop.node,
					loc: prop.node.loc,
					data: { key: prop.propName },
					messageId: "requireOptional",
					fix: canAutoFix ? (fixer) => fixer.replaceText(requiredNode, "false") : null,
					suggest: canAutoFix ? null : [{
						messageId: "fixRequiredProp",
						fix: (fixer) => fixer.replaceText(requiredNode, "false")
					}]
				});
			} else if (prop.type === "type" && defaultProps?.has(prop.propName) && prop.required) {
				if (prop.node.type === "TSMethodSignature" && (prop.node.kind === "get" || prop.node.kind === "set") || prop.node.computed) return;
				context.report({
					node: prop.node,
					loc: prop.node.loc,
					data: { key: prop.propName },
					messageId: "requireOptional",
					fix: canAutoFix ? (fixer) => fixer.insertTextAfter(prop.key, "?") : null,
					suggest: canAutoFix ? null : [{
						messageId: "fixRequiredProp",
						fix: (fixer) => fixer.insertTextAfter(prop.key, "?")
					}]
				});
			}
		};
		return require_index.default.compositingVisitors(require_index.default.defineVueVisitor(context, { onVueObjectEnter(node) {
			for (const prop of require_index.default.getComponentPropsFromOptions(node)) handleObjectProp(prop);
		} }), require_index.default.defineScriptSetupVisitor(context, { onDefinePropsEnter(node, props) {
			const defaultProps = /* @__PURE__ */ new Set([...Object.keys(require_index.default.getWithDefaultsPropExpressions(context, node).props), ...Object.keys(require_index.default.getDefaultPropExpressionsForPropsDestructure(node))]);
			for (const prop of props) handleObjectProp(prop, defaultProps);
		} }));
	}
};
exports.default = no_required_prop_with_default_default;
