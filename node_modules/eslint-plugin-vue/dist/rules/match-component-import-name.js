const require_index = require("../utils/index.js");
const require_casing = require("../utils/casing.js");
// lib/rules/match-component-import-name.ts
/**
* @author Doug Wade <douglas.b.wade@gmail.com>
* See LICENSE file in root directory for full license.
*/
function getExpectedNames(identifier) {
	return [require_casing.pascalCase(identifier.name), require_casing.kebabCase(identifier.name)];
}
var match_component_import_name_default = {
	meta: {
		type: "problem",
		docs: {
			description: "require the registered component name to match the imported component name",
			categories: void 0,
			url: "https://eslint.vuejs.org/rules/match-component-import-name.html"
		},
		fixable: null,
		schema: [],
		messages: { unexpected: "Component alias {{importedName}} should be one of: {{expectedName}}." }
	},
	create(context) {
		return require_index.default.executeOnVueComponent(context, (obj) => {
			const components = require_index.default.findProperty(obj, "components");
			if (!components || !components.value || components.value.type !== "ObjectExpression") return;
			for (const property of components.value.properties) {
				if (property.type === "SpreadElement" || property.value.type !== "Identifier" || property.computed === true) continue;
				const importedName = require_index.default.getStaticPropertyName(property) || "";
				const expectedNames = getExpectedNames(property.value);
				if (!expectedNames.includes(importedName)) context.report({
					node: property,
					messageId: "unexpected",
					data: {
						importedName,
						expectedName: expectedNames.join(", ")
					}
				});
			}
		});
	}
};
exports.default = match_component_import_name_default;
