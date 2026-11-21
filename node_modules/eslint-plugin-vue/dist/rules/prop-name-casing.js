const require_index = require("../utils/index.js");
const require_casing = require("../utils/casing.js");
const require_regexp = require("../utils/regexp.js");
// lib/rules/prop-name-casing.ts
const allowedCaseOptions = ["camelCase", "snake_case"];
function create(context) {
	const options = context.options[0];
	const isIgnoredProp = require_regexp.toRegExpGroupMatcher(context.options[1]?.ignoreProps);
	const caseType = allowedCaseOptions.includes(options) ? options : "camelCase";
	const checker = require_casing.getChecker(caseType);
	function processProps(props) {
		for (const item of props) {
			const propName = item.propName;
			if (propName == null) continue;
			if (!checker(propName) && !isIgnoredProp(propName)) context.report({
				node: item.node,
				messageId: "invalidCase",
				data: {
					name: propName,
					caseType
				}
			});
		}
	}
	return require_index.default.compositingVisitors(require_index.default.defineScriptSetupVisitor(context, { onDefinePropsEnter(_node, props) {
		processProps(props);
	} }), require_index.default.executeOnVue(context, (obj) => {
		processProps(require_index.default.getComponentPropsFromOptions(obj));
	}));
}
var prop_name_casing_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "enforce specific casing for the Prop name in Vue components",
			categories: ["vue3-strongly-recommended", "vue2-strongly-recommended"],
			url: "https://eslint.vuejs.org/rules/prop-name-casing.html"
		},
		fixable: null,
		schema: [{ enum: allowedCaseOptions }, {
			type: "object",
			properties: { ignoreProps: {
				type: "array",
				items: { type: "string" },
				uniqueItems: true
			} },
			additionalProperties: false
		}],
		messages: { invalidCase: "Prop \"{{name}}\" is not in {{caseType}}." }
	},
	create
};
exports.default = prop_name_casing_default;
