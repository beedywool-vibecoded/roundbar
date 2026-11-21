const require_index = require("../utils/index.js");
const require_vue_reserved = require("../utils/vue-reserved.json.js");
// lib/rules/no-reserved-keys.ts
/**
* @fileoverview Prevent overwrite reserved keys
* @author Armano
*/
const GROUP_NAMES = [
	"props",
	"computed",
	"data",
	"asyncData",
	"methods",
	"setup"
];
var no_reserved_keys_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "disallow overwriting reserved keys",
			categories: ["vue3-essential", "vue2-essential"],
			url: "https://eslint.vuejs.org/rules/no-reserved-keys.html"
		},
		fixable: null,
		schema: [{
			type: "object",
			properties: {
				reserved: {
					type: "array",
					items: { type: "string" }
				},
				groups: {
					type: "array",
					items: { type: "string" }
				}
			},
			additionalProperties: false
		}],
		messages: {
			reserved: "Key '{{name}}' is reserved.",
			startsWithUnderscore: "Keys starting with '_' are reserved in '{{name}}' group."
		}
	},
	create(context) {
		const options = context.options[0] || {};
		const reservedKeys = /* @__PURE__ */ new Set([...require_vue_reserved.default, ...options.reserved || []]);
		const groups = /* @__PURE__ */ new Set([...GROUP_NAMES, ...options.groups || []]);
		return require_index.default.compositingVisitors(require_index.default.defineScriptSetupVisitor(context, { onDefinePropsEnter(_node, props) {
			for (const prop of props) {
				if (!prop.propName || !reservedKeys.has(prop.propName)) continue;
				const { propName, node } = prop;
				context.report({
					node,
					messageId: "reserved",
					data: { name: propName }
				});
			}
		} }), require_index.default.executeOnVue(context, (obj) => {
			const properties = require_index.default.iterateProperties(obj, groups);
			for (const o of properties) if ((o.groupName === "data" || o.groupName === "asyncData") && o.name[0] === "_") context.report({
				node: o.node,
				messageId: "startsWithUnderscore",
				data: { name: o.name }
			});
			else if (reservedKeys.has(o.name)) context.report({
				node: o.node,
				messageId: "reserved",
				data: { name: o.name }
			});
		}));
	}
};
exports.default = no_reserved_keys_default;
