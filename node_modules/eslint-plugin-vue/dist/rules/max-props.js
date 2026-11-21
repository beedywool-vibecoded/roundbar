const require_index = require("../utils/index.js");
// lib/rules/max-props.ts
/**
* @author kevsommer Kevin Sommer
* See LICENSE file in root directory for full license.
*/
var max_props_default = {
	meta: {
		type: "problem",
		docs: {
			description: "enforce maximum number of props in Vue component",
			categories: void 0,
			url: "https://eslint.vuejs.org/rules/max-props.html"
		},
		fixable: null,
		schema: [{
			type: "object",
			properties: { maxProps: {
				type: "integer",
				minimum: 1
			} },
			additionalProperties: false,
			minProperties: 1
		}],
		messages: { tooManyProps: "Component has too many props ({{propCount}}). Maximum allowed is {{limit}}." }
	},
	create(context) {
		const option = context.options[0] || {};
		function checkMaxNumberOfProps(props, node) {
			const propCount = new Set(props.map((prop) => prop.propName)).size;
			if (propCount > option.maxProps && props[0].node) context.report({
				node,
				messageId: "tooManyProps",
				data: {
					propCount,
					limit: option.maxProps
				}
			});
		}
		return require_index.default.compositingVisitors(require_index.default.executeOnVue(context, (node) => {
			const propsNode = require_index.default.findProperty(node, "props");
			if (!propsNode) return;
			checkMaxNumberOfProps(require_index.default.getComponentPropsFromOptions(node), propsNode);
		}), require_index.default.defineScriptSetupVisitor(context, { onDefinePropsEnter(node, props) {
			checkMaxNumberOfProps(props, node);
		} }));
	}
};
exports.default = max_props_default;
