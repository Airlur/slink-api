package validator

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"unicode"

	"short-link/internal/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	en_translations "github.com/go-playground/validator/v10/translations/en"
	zh_translations "github.com/go-playground/validator/v10/translations/zh"
	
)

// Trans 定义一个全局的翻译器
var Trans ut.Translator

// InitTranslator 初始化翻译器
func InitTranslator(locale string) (err error) {
	// 修改gin框架中的Validator引擎属性，实现自定制
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		// 注册一个获取json tag的自定义方法
		v.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return ""
			}
			return name
		})

		zhT := zh.New() // 中文翻译器
		enT := en.New() // 英文翻译器

		// 第一个参数是备用（fallback）的语言环境
		// 后面的参数是应该支持的语言环境（支持多个）
		uni := ut.New(enT, zhT, enT)

		// locale 通常取决于 http 请求头的 'Accept-Language'
		var ok bool
		Trans, ok = uni.GetTranslator(locale)
		if !ok {
			return fmt.Errorf("uni.GetTranslator(%s) failed", locale)
		}

		// 注册翻译器
		switch locale {
		case "en":
			err = en_translations.RegisterDefaultTranslations(v, Trans)
		case "zh":
			err = zh_translations.RegisterDefaultTranslations(v, Trans)
		default:
			err = en_translations.RegisterDefaultTranslations(v, Trans)
		}
		if err != nil {
			return err
		}

		// 【核心新增点】在这里注册我们的自定义校验函数
		// 步骤1：注册自定义的 "password" 校验规则，并关联我们的实现函数
		if err = v.RegisterValidation("password", passwordValidator); err != nil {
			return err
		}
		// 步骤2：为这个新的 'password' 标签注册中文翻译
		if err = v.RegisterTranslation("password", Trans, func(ut ut.Translator) error {
			return ut.Add("password", "{0}必须包含大写字母、小写字母、数字和特殊字符", true)
		}, func(ut ut.Translator, fe validator.FieldError) string {
			t, _ := ut.T("password", fe.Field())
			return t
		}); err != nil {
			return err
		}
		
		return nil
	}
	return nil
}

// passwordValidator 是自定义的密码复杂度校验函数
// 它实现了 validator.Func 接口，用于被 RegisterValidation 调用
func passwordValidator(fl validator.FieldLevel) bool {
	password, ok := fl.Field().Interface().(string)
	if !ok {
		return false
	}
	
	var (
		hasUpper   = false // 是否有大写字母
		hasLower   = false // 是否有小写字母
		hasNumber  = false // 是否有数字
		hasSpecial = false // 是否有特殊字符
	)

	// 遍历密码字符串中的每一个字符
	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		// IsPunct 检查标点符号, IsSymbol 检查其他符号
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}
	// 必须同时满足所有条件
	return hasUpper && hasLower && hasNumber && hasSpecial
}

// TranslateValidationErrors 将验证错误翻译成map
func TranslateValidationErrors(err error) map[string]string {
	errs, ok := err.(validator.ValidationErrors)
	if !ok {
		return nil
	}
	return errs.Translate(Trans)
}

// HandleBindingError 是一个公共的参数绑定错误处理器，专门处理 Gin 的参数绑定和校验错误
func HandleBindingError(c *gin.Context, err error) {
	// 场景1：validator的binding标签校验失败
	var verr validator.ValidationErrors
	if errors.As(err, &verr) {
		// 使用我们自定义的翻译器
		errs := TranslateValidationErrors(verr)
		var errMsg strings.Builder
		for _, v := range errs {
			errMsg.WriteString(v + "; ")
		}
		response.Fail(c, response.InvalidParam, strings.TrimRight(errMsg.String(), "; "))
		return
	}

	// 场景2：JSON本身的类型不匹配
	var jerr *json.UnmarshalTypeError
	if errors.As(err, &jerr) {
		msg := fmt.Sprintf("字段 '%s' 的类型错误，期望是 %s 类型", jerr.Field, jerr.Type.String())
		response.Fail(c, response.InvalidParam, msg)
		return
	}
	
	// 场景3：其他绑定错误（例如空的请求体）
	response.Fail(c, response.InvalidParam, "参数绑定失败: "+err.Error())
}