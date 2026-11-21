const require_index = require("../utils/index.js");
// lib/rules/prefer-use-template-ref.ts
/**
* @author Thomasan1999
* See LICENSE file in root directory for full license.
*/
function getScriptRefsFromSetupFunction(body) {
	return body.flatMap((child) => {
		if (child.type === "VariableDeclaration") {
			const declarator = child.declarations[0];
			if (declarator.init?.type === "CallExpression" && declarator.init.callee?.type === "Identifier" && declarator.id.type === "Identifier" && ["ref", "shallowRef"].includes(declarator.init.callee.name)) return [[declarator.id.name, declarator.init]];
		}
		return [];
	});
}
var prefer_use_template_ref_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "require using `useTemplateRef` instead of `ref`/`shallowRef` for template refs",
			categories: void 0,
			url: "https://eslint.vuejs.org/rules/prefer-use-template-ref.html"
		},
		schema: [],
		messages: { preferUseTemplateRef: "Replace '{{name}}' with 'useTemplateRef'." }
	},
	create(context) {
		const templateRefs = /* @__PURE__ */ new Set();
		const scriptRefs = [];
		return require_index.default.compositingVisitors(require_index.default.defineTemplateBodyVisitor(context, { "VAttribute[directive=false]"(node) {
			if (node.key.name === "ref" && node.value?.value) templateRefs.add(node.value.value);
		} }), require_index.default.defineVueVisitor(context, { onSetupFunctionEnter(node) {
			if (node.type === "ArrowFunctionExpression" && node.expression) return;
			const newScriptRefs = getScriptRefsFromSetupFunction(node.body.body);
			scriptRefs.push(...newScriptRefs);
		} }), require_index.default.defineScriptSetupVisitor(context, { Program(node) {
			const newScriptRefs = getScriptRefsFromSetupFunction(node.body);
			scriptRefs.push(...newScriptRefs);
		} }), { "Program:exit"() {
			const scriptRefsMap = new Map(scriptRefs);
			for (const templateRef of templateRefs) {
				const scriptRef = scriptRefsMap.get(templateRef);
				if (!scriptRef) continue;
				context.report({
					node: scriptRef,
					messageId: "preferUseTemplateRef",
					data: { name: scriptRef.callee.name }
				});
			}
		} });
	}
};
exports.default = prefer_use_template_ref_default;
