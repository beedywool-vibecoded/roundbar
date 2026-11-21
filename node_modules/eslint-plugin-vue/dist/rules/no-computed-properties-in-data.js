const require_index = require("../utils/index.js");
// lib/rules/no-computed-properties-in-data.ts
/**
* @author  Yosuke Ota
* See LICENSE file in root directory for full license.
*/
var no_computed_properties_in_data_default = {
	meta: {
		type: "problem",
		docs: {
			description: "disallow accessing computed properties in `data`",
			categories: ["vue3-essential", "vue2-essential"],
			url: "https://eslint.vuejs.org/rules/no-computed-properties-in-data.html"
		},
		fixable: null,
		schema: [],
		messages: { cannotBeUsed: "The computed property cannot be used in `data()` because it is before initialization." }
	},
	create(context) {
		const contextMap = /* @__PURE__ */ new Map();
		let scopeStack = null;
		return require_index.default.compositingVisitors({
			":function"(node) {
				scopeStack = {
					upper: scopeStack,
					node
				};
			},
			":function:exit"() {
				scopeStack &&= scopeStack.upper;
			}
		}, require_index.default.defineVueVisitor(context, {
			onVueObjectEnter(node) {
				const dataProperty = require_index.default.findProperty(node, "data");
				if (!dataProperty || dataProperty.value.type !== "FunctionExpression" && dataProperty.value.type !== "ArrowFunctionExpression") return;
				const computedNames = /* @__PURE__ */ new Set();
				const computedProperties = require_index.default.iterateProperties(node, /* @__PURE__ */ new Set(["computed"]));
				for (const computed of computedProperties) computedNames.add(computed.name);
				contextMap.set(node, {
					data: dataProperty.value,
					computedNames
				});
			},
			MemberExpression(node, vueData) {
				if (!scopeStack || !require_index.default.isThis(node.object, context)) return;
				const ctx = contextMap.get(vueData.node);
				if (!ctx || ctx.data !== scopeStack.node) return;
				const name = require_index.default.getStaticPropertyName(node);
				if (!name || !ctx.computedNames.has(name)) return;
				context.report({
					node,
					messageId: "cannotBeUsed"
				});
			}
		}));
	}
};
exports.default = no_computed_properties_in_data_default;
