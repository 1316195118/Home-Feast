package api

import "testing"

func TestNormalizeDishDefaultsToTwoServings(t *testing.T) {
	dish, err := normalizeDish(dishInput{
		Name:        "测试豆腐",
		Cuisine:     "homestyle",
		Tags:        []string{"家常", "素菜"},
		Ingredients: []string{"嫩豆腐 200g"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if dish.Servings != 2 {
		t.Fatalf("未填写份数时应按 2 人份，实际 %d", dish.Servings)
	}
}

func TestNormalizeDishKeepsExplicitServings(t *testing.T) {
	dish, err := normalizeDish(dishInput{
		Name:        "测试炖菜",
		Cuisine:     "homestyle",
		Tags:        []string{"家常", "荤菜"},
		Servings:    5,
		Ingredients: []string{"牛肉 800g"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if dish.Servings != 5 {
		t.Fatalf("显式份数应保留，实际 %d", dish.Servings)
	}
}
