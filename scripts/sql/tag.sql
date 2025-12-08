-- 短链标签表（tags）
CREATE TABLE IF NOT EXISTS `tags` (
  `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '标签唯一标识',
  `short_code` VARCHAR(20) NOT NULL COMMENT '关联短码',
  `user_id` BIGINT NOT NULL COMMENT '关联用户ID',
  `tag_name` VARCHAR(30) NOT NULL COMMENT '标签名称（如“活动推广”）',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `deleted_at` DATETIME NULL DEFAULT NULL COMMENT '软删除标记（非NULL表示已删除）',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_short_tag` (`user_id`, `short_code`, `tag_name`),
  INDEX `idx_tag_name` (`tag_name`),
  INDEX `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='短链接标签表';