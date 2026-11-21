const require_index = require("../utils/index.js");
// lib/rules/no-deprecated-v-on-native-modifier.ts
/**
* @author Yosuke Ota
* See LICENSE file in root directory for full license.
*/
var no_deprecated_v_on_native_modifier_default = {
	meta: {
		type: "problem",
		docs: {
			description: "disallow using deprecated `.native` modifiers (in Vue.js 3.0.0+)",
			categories: ["vue3-essential"],
			url: "https://eslint.vuejs.org/rules/no-deprecated-v-on-native-modifier.html"
		},
		fixable: null,
		schema: [],
		messages: { deprecated: "'.native' modifier on 'v-on' directive is deprecated." }
	},
	create(context) {
		return require_index.default.defineTemplateBodyVisitor(context, { "VAttribute[directive=true][key.name.name='on'] > VDirectiveKey > VIdentifier[name='native']"(node) {
			if (!node.parent.modifiers.includes(node)) return;
			context.report({
				node,
				messageId: "deprecated"
			});
		} });
	}
};
exports.default = no_deprecated_v_on_native_modifier_default;
